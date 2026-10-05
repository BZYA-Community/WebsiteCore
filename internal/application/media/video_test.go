package media

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	mp4 "github.com/abema/go-mp4"
)

func TestVideoRejectsForgedContainerDimensions(t *testing.T) {
	data := videoFixture(t, "2560x1440", "30000/1001", "libx264", false)
	// VisualSampleEntry width/height are 28 bytes after its fourcc.
	i := bytes.Index(data, []byte("avc1")) // ftyp can also contain avc1
	for i >= 0 && (i+32 > len(data) || binary.BigEndian.Uint16(data[i+28:i+30]) != 2560) {
		next := bytes.Index(data[i+4:], []byte("avc1"))
		if next < 0 {
			i = -1
			break
		}
		i += 4 + next
	}
	if i < 0 {
		t.Fatal("missing 1440p sample entry")
	}
	binary.BigEndian.PutUint16(data[i+28:i+30], 640)
	binary.BigEndian.PutUint16(data[i+30:i+32], 360)
	if err := ValidateVideo(t.Context(), bytes.NewReader(data), VideoLimit); err == nil {
		t.Fatal("1440p H.264 accepted after only changing four container bytes")
	}
}

// FFmpeg creates independent test fixtures; it is not a server dependency.
func videoFixture(t *testing.T, dimensions, rate, codec string, audio bool, extra ...string) []byte {
	t.Helper()
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		if os.Getenv("REQUIRE_FFMPEG_TESTS") == "1" {
			t.Fatal("ffmpeg required to generate test fixtures")
		}
		t.Skip("ffmpeg unavailable; set REQUIRE_FFMPEG_TESTS=1 in CI")
	}
	path := filepath.Join(t.TempDir(), "fixture.mp4")
	args := []string{"-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "testsrc2=size=" + dimensions + ":rate=" + rate}
	if audio {
		args = append(args, "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000")
	}
	args = append(args, "-t", "0.6", "-c:v", codec, "-threads", "2", "-c:a", "aac")
	args = append(args, extra...)
	args = append(args, path)
	if output, err := exec.CommandContext(t.Context(), ffmpeg, args...).CombinedOutput(); err != nil {
		t.Fatalf("fixture: %v: %s", err, output)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func fixtureBox(t *testing.T, data []byte, name string) mp4.BoxInfo {
	t.Helper()
	var found *mp4.BoxInfo
	err := walkVideoBoxes(bytes.NewReader(data), uint64(len(data)), nil, func(h *mp4.ReadHandle) (interface{}, error) {
		kind := h.BoxInfo.Type.String()
		if kind == name {
			bi := h.BoxInfo
			found = &bi
		}
		switch kind {
		case "moov", "trak", "mdia", "minf", "stbl":
			return h.Expand()
		case "stsd", "avc1":
			if _, _, err := h.ReadPayload(); err != nil {
				return nil, err
			}
			return h.Expand()
		}
		return nil, nil
	})
	if err != nil || found == nil {
		t.Fatalf("find %s: %v", name, err)
	}
	return *found
}

func fixtureSPS(t *testing.T, data []byte) []byte {
	t.Helper()
	bi := fixtureBox(t, data, "avcC")
	config := data[bi.Offset+bi.HeaderSize : bi.Offset+bi.Size]
	length := int(binary.BigEndian.Uint16(config[6:8]))
	return config[8 : 8+length]
}

func TestVideoH264Profiles(t *testing.T) {
	for _, tc := range []struct {
		name  string
		extra []string
	}{
		{"baseline", []string{"-profile:v", "baseline"}},
		{"main", []string{"-profile:v", "main"}},
		{"high 420", []string{"-profile:v", "high"}},
		{"high 422", []string{"-pix_fmt", "yuv422p", "-profile:v", "high422"}},
		{"high 444", []string{"-pix_fmt", "yuv444p", "-profile:v", "high444"}},
		{"interlaced", []string{"-flags", "+ilme+ildct"}},
		{"scaling matrix", []string{"-x264-params", "cqm=jvt"}},
		{"repeated SPS", []string{"-x264-params", "repeat-headers=1"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := videoFixture(t, "640x360", "30000/1001", "libx264", false, tc.extra...)
			if err := ValidateVideo(t.Context(), bytes.NewReader(data), VideoLimit); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestVideoRejectsDamagedSampleTables(t *testing.T) {
	fixture := videoFixture(t, "640x360", "30000/1001", "libx264", false)
	for _, kind := range []string{"stsz", "stsc", "stco"} {
		t.Run(kind, func(t *testing.T) {
			data := append([]byte(nil), fixture...)
			bi := fixtureBox(t, data, kind)
			// A table row claiming billions of frames or a chunk outside mdat
			// must fail without allocating memory based on the untrusted count.
			pos := bi.Offset + bi.HeaderSize + 8
			binary.BigEndian.PutUint32(data[pos:pos+4], ^uint32(0))
			if err := ValidateVideo(t.Context(), bytes.NewReader(data), VideoLimit); err == nil {
				t.Fatal("bad table accepted")
			}
		})
	}
}

func TestVideoAVCConfigValidatesEverySPS(t *testing.T) {
	small := fixtureSPS(t, videoFixture(t, "640x360", "30000/1001", "libx264", false))
	large := fixtureSPS(t, videoFixture(t, "2560x1440", "30000/1001", "libx264", false))
	for _, tc := range []struct {
		name string
		sets [][]byte
		ok   bool
	}{
		{"two valid", [][]byte{small, small}, true},
		{"second oversized", [][]byte{small, large}, false},
		{"missing", nil, false},
		{"truncated", [][]byte{small[:4]}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config := []byte{1, 100, 0, 31, 255, 224 | byte(len(tc.sets))}
			for _, sps := range tc.sets {
				config = binary.BigEndian.AppendUint16(config, uint16(len(sps)))
				config = append(config, sps...)
			}
			config = append(config, 1, 0, 2, 0x68, 0x80) // one PPS
			track := videoTrack{width: 640, height: 360}
			if err := track.readAVCConfig(config); (err == nil) != tc.ok {
				t.Fatalf("config: %v", err)
			}
		})
	}
}

func TestVideoSampleCannotReplaceSPSWithOversizedOne(t *testing.T) {
	small := fixtureSPS(t, videoFixture(t, "640x360", "30000/1001", "libx264", false))
	large := fixtureSPS(t, videoFixture(t, "2560x1440", "30000/1001", "libx264", false))
	for _, length := range []int{1, 2, 4} {
		for _, sps := range [][]byte{small, large} {
			var sample []byte
			for _, nal := range [][]byte{sps, {0x65, 0x80}} {
				var n [4]byte
				binary.BigEndian.PutUint32(n[:], uint32(len(nal)))
				sample = append(sample, n[4-length:]...)
				sample = append(sample, nal...)
			}
			track := videoTrack{width: 640, height: 360, nalLength: length}
			err := track.validateSample(t.Context(), bytes.NewReader(sample), 0, uint64(len(sample)))
			if (err == nil) != bytes.Equal(sps, small) {
				t.Fatalf("inline SPS length=%d: %v", length, err)
			}
			if err := track.validateSample(t.Context(), bytes.NewReader(sample[:len(sample)-1]), 0, uint64(len(sample)-1)); err == nil {
				t.Fatal("truncated NAL accepted")
			}
		}
	}
}

func TestVideoAllowsWebCodecsPrefixNAL(t *testing.T) {
	// A hardware WebCodecs encoder emits this temporal-layer prefix before
	// ordinary AVC slices. Prefix metadata does not set picture dimensions.
	prefix := []byte{0x6e, 0xc0, 0x80, 0x0f, 0x20}
	for _, kind := range []byte{1, 5, 20, 21} {
		var sample []byte
		for _, nal := range [][]byte{prefix, {0x60 | kind, 0x80}} {
			sample = binary.BigEndian.AppendUint32(sample, uint32(len(nal)))
			sample = append(sample, nal...)
		}
		track := videoTrack{width: 640, height: 360, nalLength: 4}
		err := track.validateSample(t.Context(), bytes.NewReader(sample), 0, uint64(len(sample)))
		if (err == nil) != (kind == 1 || kind == 5) {
			t.Fatalf("slice type %d: %v", kind, err)
		}
	}
}

func TestValidateVideo(t *testing.T) {
	for _, tc := range []struct {
		name, dimensions, rate, codec string
		audio, ok                     bool
	}{
		{"landscape", "1920x1080", "30000/1001", "libx264", true, true},
		{"portrait", "1080x1920", "30000/1001", "libx264", false, true},
		{"small", "640x360", "30000/1001", "libx264", true, true},
		{"high resolution", "2560x1440", "30000/1001", "libx264", false, false},
		{"square over 1080", "1200x1200", "30000/1001", "libx264", false, false},
		{"30 is not 29.97", "640x360", "30", "libx264", false, false},
		{"wrong codec", "640x360", "30000/1001", "mpeg4", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := videoFixture(t, tc.dimensions, tc.rate, tc.codec, tc.audio)
			reader := bytes.NewReader(data)
			if err := ValidateVideo(t.Context(), reader, VideoLimit); (err == nil) != tc.ok {
				t.Fatalf("validation: %v", err)
			}
			if pos, _ := reader.Seek(0, io.SeekCurrent); pos != 0 {
				t.Fatal("validation must rewind file")
			}
			if err := ValidateVideo(t.Context(), reader, int64(len(data)-1)); !errors.Is(err, ErrTooLarge) {
				t.Fatal(err)
			}
			if err := ValidateVideo(t.Context(), bytes.NewReader(data[:len(data)/2]), VideoLimit); err == nil {
				t.Fatal("truncated container accepted")
			}
		})
	}
}

func TestVideoInvalidAndCanceled(t *testing.T) {
	for _, data := range [][]byte{nil, []byte("not MP4"), {0, 0, 0, 4, 'f', 't', 'y', 'p'}} {
		if err := ValidateVideo(t.Context(), bytes.NewReader(data), VideoLimit); err == nil {
			t.Fatal("invalid accepted")
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := ValidateVideo(ctx, bytes.NewReader([]byte("12345678")), VideoLimit); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
