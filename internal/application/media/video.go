package media

import (
	"context"
	"encoding/binary"
	"io"
	"math"

	mp4 "github.com/abema/go-mp4"
)

type videoTrack struct {
	handler                                 string
	timescale                               uint32
	width, height                           uint16
	avc, config, timing                     bool
	minDelta, maxDelta                      uint32
	samples                                 uint64
	ticks                                   uint64
	nalLength                               int
	sampleSizes, sampleChunks, chunkOffsets *mp4.BoxInfo
}

// ValidateVideo checks the MP4 container, AVC parameter sets, sample bounds and
// constant frame timing without loading the video into RAM or running a server
// decoder. Browser output is a non-fragmented MP4 with a primary video track.
func ValidateVideo(ctx context.Context, source io.ReadSeeker, limit int64) error {
	size, err := checkSize(source, limit)
	if err != nil {
		return err
	}
	defer func() { _, _ = source.Seek(0, io.SeekStart) }()
	// Bound boxes before go-mp4 automatically unmarshals ftyp.
	var haveFTYP, haveMDAT bool
	var mediaRanges []videoRange
	topLevelBoxes := 0
	for offset := int64(0); offset < size; {
		topLevelBoxes++
		if topLevelBoxes > 10000 {
			return ErrInvalid
		}
		if err = ctx.Err(); err != nil {
			return err
		}
		bi, readErr := mp4.ReadBoxInfo(source)
		if readErr != nil || bi.Size < bi.HeaderSize || bi.Size > uint64(size-offset) {
			return ErrInvalid
		}
		switch bi.Type.String() {
		case "ftyp":
			if haveFTYP || bi.Size > 1024 {
				return ErrInvalid
			}
			haveFTYP = true
		case "mdat":
			haveMDAT = bi.Size > bi.HeaderSize
			mediaRanges = append(mediaRanges, videoRange{bi.Offset + bi.HeaderSize, bi.Offset + bi.Size})
		case "moof", "keys":
			return ErrInvalid
		}
		offset += int64(bi.Size)
		if _, err = source.Seek(offset, io.SeekStart); err != nil {
			return err
		}
	}
	if !haveFTYP || !haveMDAT {
		return ErrInvalid
	}
	if _, err = source.Seek(0, io.SeekStart); err != nil {
		return err
	}
	var tracks []*videoTrack
	boxCount := 0
	err = walkVideoBoxes(source, uint64(size), nil, func(h *mp4.ReadHandle) (interface{}, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		boxCount++
		if boxCount > 10000 || h.BoxInfo.Size < h.BoxInfo.HeaderSize {
			return nil, ErrInvalid
		}
		path := ""
		for i, p := range h.Path {
			if i > 0 {
				path += "/"
			}
			path += p.String()
		}
		var track *videoTrack
		if len(h.Params) > 0 {
			track, _ = h.Params[0].(*videoTrack)
		}
		switch path {
		case "moov":
			return h.Expand()
		case "moov/trak":
			track = &videoTrack{}
			tracks = append(tracks, track)
			return h.Expand(track)
		case "moov/trak/mdia", "moov/trak/mdia/minf", "moov/trak/mdia/minf/stbl":
			return h.Expand(track)
		case "moov/trak/mdia/hdlr", "moov/trak/mdia/mdhd", "moov/trak/mdia/minf/stbl/stsd", "moov/trak/mdia/minf/stbl/stsd/avc1":
			// Only small metadata structures are unmarshalled.
			if h.BoxInfo.Size > 65536 {
				return nil, ErrInvalid
			}
			box, _, err := h.ReadPayload()
			if err != nil {
				return nil, ErrInvalid
			}
			switch b := box.(type) {
			case *mp4.Hdlr:
				track.handler = string(b.HandlerType[:])
			case *mp4.Mdhd:
				track.timescale = b.Timescale
			case *mp4.Stsd:
				if b.EntryCount != 1 {
					return nil, ErrInvalid
				}
				return h.Expand(track)
			case *mp4.VisualSampleEntry:
				track.avc, track.width, track.height = true, b.Width, b.Height
				return h.Expand(track)
			}
		case "moov/trak/mdia/minf/stbl/stsd/avc1/avcC":
			if track.config || h.BoxInfo.Size > 65536 {
				return nil, ErrInvalid
			}
			payload := make([]byte, h.BoxInfo.Size-h.BoxInfo.HeaderSize)
			if _, err := io.ReadFull(source, payload); err != nil {
				return nil, ErrInvalid
			}
			if err := track.readAVCConfig(payload); err != nil {
				return nil, err
			}
			track.config = true
		case "moov/trak/mdia/minf/stbl/stsz", "moov/trak/mdia/minf/stbl/stsc",
			"moov/trak/mdia/minf/stbl/stco", "moov/trak/mdia/minf/stbl/co64":
			field := &track.chunkOffsets
			switch h.BoxInfo.Type.String() {
			case "stsz":
				field = &track.sampleSizes
			case "stsc":
				field = &track.sampleChunks
			}
			if *field != nil {
				return nil, ErrInvalid
			}
			bi := h.BoxInfo
			*field = &bi
		case "moov/trak/mdia/minf/stbl/stts":
			// Stream the run-length table rather than allocating an entry per frame.
			var header [8]byte
			if _, err := io.ReadFull(source, header[:]); err != nil {
				return nil, ErrInvalid
			}
			count := binary.BigEndian.Uint32(header[4:])
			if uint64(count)*8+8 != h.BoxInfo.Size-h.BoxInfo.HeaderSize || count == 0 {
				return nil, ErrInvalid
			}
			track.timing = true
			for range count {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				if _, err := io.ReadFull(source, header[:]); err != nil {
					return nil, ErrInvalid
				}
				n, delta := binary.BigEndian.Uint32(header[:4]), binary.BigEndian.Uint32(header[4:])
				if n == 0 || delta == 0 {
					return nil, ErrInvalid
				}
				track.samples += uint64(n)
				track.ticks += uint64(n) * uint64(delta)
				if track.minDelta == 0 || delta < track.minDelta {
					track.minDelta = delta
				}
				if delta > track.maxDelta {
					track.maxDelta = delta
				}
				if track.samples > uint64(size) {
					return nil, ErrInvalid
				}
			}
		}
		return nil, nil
	})
	if err != nil {
		return err
	}
	videoCount := 0
	for _, t := range tracks {
		if t.handler != "vide" {
			continue
		}
		videoCount++
		long, short := t.width, t.height
		if short > long {
			long, short = short, long
		}
		if !t.avc || !t.config || !t.timing || t.timescale == 0 || t.samples == 0 || short == 0 || long > 1920 || short > 1080 {
			return ErrInvalid
		}
		expected := float64(t.timescale) * 1001 / 30000
		if math.Abs(float64(t.minDelta)-expected) > 1 || math.Abs(float64(t.maxDelta)-expected) > 1 {
			return ErrInvalid
		}
		// Allow timestamp rounding, not a 30 fps stream mislabelled as 29.97.
		if math.Abs(float64(t.ticks)-float64(t.samples)*float64(t.timescale)*1001/30000) > 1.01 {
			return ErrInvalid
		}
		if err := t.validateSamples(ctx, source, mediaRanges); err != nil {
			return err
		}
	}
	if videoCount != 1 {
		return ErrInvalid
	}
	return nil
}

// walkVideoBoxes uses the library's box reader, but only unmarshals payloads
// explicitly requested by our handler. ReadBoxStructure also automatically
// decodes arbitrary ftyp/keys boxes; those are unnecessary for validation and
// can contain attacker-controlled lists. Parent bounds apply at every depth.
func walkVideoBoxes(r io.ReadSeeker, end uint64, path mp4.BoxPath, handler mp4.ReadHandler, params ...interface{}) error {
	for {
		offset, err := r.Seek(0, io.SeekCurrent)
		if err != nil {
			return err
		}
		if uint64(offset) == end {
			return nil
		}
		if offset < 0 || uint64(offset) > end || end-uint64(offset) < 8 {
			return ErrInvalid
		}
		bi, err := mp4.ReadBoxInfo(r)
		if err != nil || bi.Size < bi.HeaderSize || bi.Size > end-uint64(offset) {
			return ErrInvalid
		}
		childPath := append(append(mp4.BoxPath{}, path...), bi.Type)
		payloadStart := bi.Offset + bi.HeaderSize
		childrenStart := payloadStart
		h := &mp4.ReadHandle{BoxInfo: *bi, Path: childPath, Params: params}
		h.ReadPayload = func() (mp4.IBox, uint64, error) {
			if _, err := r.Seek(int64(payloadStart), io.SeekStart); err != nil {
				return nil, 0, err
			}
			box, n, err := mp4.UnmarshalAny(r, bi.Type, bi.Size-bi.HeaderSize, bi.Context)
			childrenStart = payloadStart + n
			return box, n, err
		}
		h.Expand = func(p ...interface{}) ([]interface{}, error) {
			if len(childPath) > 8 || childrenStart > bi.Offset+bi.Size {
				return nil, ErrInvalid
			}
			if _, err := r.Seek(int64(childrenStart), io.SeekStart); err != nil {
				return nil, err
			}
			return nil, walkVideoBoxes(r, bi.Offset+bi.Size, childPath, handler, p...)
		}
		if _, err := handler(h); err != nil {
			return err
		}
		if _, err := r.Seek(int64(bi.Offset+bi.Size), io.SeekStart); err != nil {
			return err
		}
	}
}
