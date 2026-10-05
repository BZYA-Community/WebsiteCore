package media

import (
	"context"
	"encoding/binary"
	"io"
	"sort"

	mp4 "github.com/abema/go-mp4"
)

func (t *videoTrack) validateSPS(nal []byte) error {
	w, h, err := h264Dimensions(nal)
	if err != nil || w != t.width || h != t.height {
		return ErrInvalid
	}
	return nil
}

func (t *videoTrack) readAVCConfig(data []byte) error {
	if len(data) < 7 || data[0] != 1 || data[4]&0xfc != 0xfc || data[5]&0xe0 != 0xe0 {
		return ErrInvalid
	}
	t.nalLength = int(data[4]&3) + 1
	if t.nalLength == 3 {
		return ErrInvalid
	}
	count := int(data[5] & 31)
	if count == 0 {
		return ErrInvalid
	}
	pos := 6
	readNAL := func() ([]byte, error) {
		if pos+2 > len(data) {
			return nil, ErrInvalid
		}
		size := int(binary.BigEndian.Uint16(data[pos:]))
		pos += 2
		if size == 0 || size > len(data)-pos {
			return nil, ErrInvalid
		}
		nal := data[pos : pos+size]
		pos += size
		return nal, nil
	}
	for range count {
		nal, err := readNAL()
		if err != nil || t.validateSPS(nal) != nil {
			return ErrInvalid
		}
	}
	if pos >= len(data) || data[pos] == 0 {
		return ErrInvalid
	}
	count = int(data[pos])
	pos++
	for range count {
		nal, err := readNAL()
		if err != nil || nal[0]&0x9f != 8 {
			return ErrInvalid
		}
	}
	// High-profile avcC may include chroma/bit-depth fields. Scalable and
	// auxiliary SPS extensions are outside the browser AVC output contract.
	if pos != len(data) && (len(data)-pos != 4 || data[pos]&0xfc != 0xfc ||
		data[pos+1]&0xf8 != 0xf8 || data[pos+2]&0xf8 != 0xf8 || data[pos+3] != 0) {
		return ErrInvalid
	}
	return nil
}

type videoRange struct{ start, end uint64 }
type videoTable struct {
	offset          uint64
	count, constant uint32
	width           int
}

// Keep table locations, not one allocation per frame of a potentially 1 GiB
// upload. Validate counts against the containing box before seeking entries.
func readVideoTable(r io.ReadSeeker, bi *mp4.BoxInfo) (videoTable, error) {
	if bi == nil {
		return videoTable{}, ErrInvalid
	}
	headerSize, width := 8, 4
	switch bi.Type.String() {
	case "stsz":
		headerSize = 12
	case "stsc":
		width = 12
	case "co64":
		width = 8
	}
	if bi.Size-bi.HeaderSize < uint64(headerSize) {
		return videoTable{}, ErrInvalid
	}
	var header [12]byte
	if _, err := r.Seek(int64(bi.Offset+bi.HeaderSize), io.SeekStart); err != nil {
		return videoTable{}, err
	}
	if _, err := io.ReadFull(r, header[:headerSize]); err != nil {
		return videoTable{}, ErrInvalid
	}
	if binary.BigEndian.Uint32(header[:4]) != 0 {
		return videoTable{}, ErrInvalid
	}
	table := videoTable{offset: bi.Offset + bi.HeaderSize + uint64(headerSize), width: width}
	table.count = binary.BigEndian.Uint32(header[headerSize-4 : headerSize])
	if headerSize == 12 {
		table.constant = binary.BigEndian.Uint32(header[4:8])
	}
	bytes := uint64(table.count) * uint64(width)
	if table.constant != 0 {
		bytes = 0
	}
	if table.count == 0 || uint64(headerSize)+bytes != bi.Size-bi.HeaderSize {
		return videoTable{}, ErrInvalid
	}
	return table, nil
}

func (t videoTable) entry(r io.ReadSeeker, index uint32) ([12]byte, error) {
	var data [12]byte
	if index >= t.count {
		return data, ErrInvalid
	}
	if t.constant != 0 {
		binary.BigEndian.PutUint32(data[:], t.constant)
		return data, nil
	}
	if _, err := r.Seek(int64(t.offset+uint64(index)*uint64(t.width)), io.SeekStart); err != nil {
		return data, err
	}
	_, err := io.ReadFull(r, data[:t.width])
	return data, err
}

func (t *videoTrack) validateSamples(ctx context.Context, r io.ReadSeeker, ranges []videoRange) error {
	sizes, err := readVideoTable(r, t.sampleSizes)
	if err != nil || uint64(sizes.count) != t.samples {
		return ErrInvalid
	}
	chunks, err := readVideoTable(r, t.chunkOffsets)
	if err != nil || chunks.count > sizes.count {
		return ErrInvalid
	}
	mapping, err := readVideoTable(r, t.sampleChunks)
	if err != nil || mapping.count > chunks.count {
		return ErrInvalid
	}
	var sample, mapIndex, perChunk uint32
	nextChunk := uint64(1)
	var previousEnd uint64
	for chunk := uint32(0); chunk < chunks.count; chunk++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		if uint64(chunk)+1 == nextChunk {
			entry, err := mapping.entry(r, mapIndex)
			if err != nil || binary.BigEndian.Uint32(entry[:4]) != chunk+1 ||
				binary.BigEndian.Uint32(entry[8:12]) != 1 {
				return ErrInvalid
			}
			perChunk = binary.BigEndian.Uint32(entry[4:8])
			mapIndex++
			nextChunk = uint64(chunks.count) + 1
			if mapIndex < mapping.count {
				next, err := mapping.entry(r, mapIndex)
				if err != nil {
					return ErrInvalid
				}
				nextChunk = uint64(binary.BigEndian.Uint32(next[:4]))
				if nextChunk <= uint64(chunk)+1 || nextChunk > uint64(chunks.count) {
					return ErrInvalid
				}
			}
		}
		if perChunk == 0 || perChunk > sizes.count-sample {
			return ErrInvalid
		}
		entry, err := chunks.entry(r, chunk)
		if err != nil {
			return ErrInvalid
		}
		offset := uint64(binary.BigEndian.Uint32(entry[:4]))
		if chunks.width == 8 {
			offset = binary.BigEndian.Uint64(entry[:8])
		}
		if offset < previousEnd {
			return ErrInvalid
		}
		region := sort.Search(len(ranges), func(i int) bool { return ranges[i].end > offset })
		if region == len(ranges) || offset < ranges[region].start {
			return ErrInvalid
		}
		for range perChunk {
			if err := ctx.Err(); err != nil {
				return err
			}
			sizeEntry, err := sizes.entry(r, sample)
			if err != nil {
				return ErrInvalid
			}
			size := uint64(binary.BigEndian.Uint32(sizeEntry[:4]))
			if size == 0 || size > ranges[region].end-offset {
				return ErrInvalid
			}
			if err := t.validateSample(ctx, r, offset, size); err != nil {
				return err
			}
			offset += size
			sample++
		}
		previousEnd = offset
	}
	if sample != sizes.count || mapIndex != mapping.count {
		return ErrInvalid
	}
	return nil
}

func (t *videoTrack) validateSample(ctx context.Context, r io.ReadSeeker, offset, size uint64) error {
	end := offset + size
	haveSlice := false
	for offset < end {
		if err := ctx.Err(); err != nil {
			return err
		}
		if end-offset <= uint64(t.nalLength) {
			return ErrInvalid
		}
		if _, err := r.Seek(int64(offset), io.SeekStart); err != nil {
			return err
		}
		var length [4]byte
		if _, err := io.ReadFull(r, length[4-t.nalLength:]); err != nil {
			return ErrInvalid
		}
		n := uint64(binary.BigEndian.Uint32(length[:]))
		offset += uint64(t.nalLength)
		if n == 0 || n > end-offset {
			return ErrInvalid
		}
		var header [1]byte
		if _, err := io.ReadFull(r, header[:]); err != nil || header[0]&0x80 != 0 {
			return ErrInvalid
		}
		switch header[0] & 31 {
		case 1, 5:
			haveSlice = true
		case 7:
			if n > 65535 {
				return ErrInvalid
			}
			nal := make([]byte, int(n))
			nal[0] = header[0]
			if _, err := io.ReadFull(r, nal[1:]); err != nil || t.validateSPS(nal) != nil {
				return ErrInvalid
			}
		case 6, 8, 9, 10, 11, 12, 14:
			// Non-slice AVC metadata; none can change coded dimensions.
			// WebCodecs hardware encoders can emit prefix NALs (type 14)
			// alongside ordinary AVC slices. Extension slices remain rejected.
		default:
			return ErrInvalid
		}
		offset += n
	}
	if !haveSlice {
		return ErrInvalid
	}
	return nil
}
