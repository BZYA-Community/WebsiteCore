package media

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// FFmpeg creates independent test fixtures; it is not a server dependency.
func videoFixture(t *testing.T, dimensions, rate, codec string, audio bool) []byte {
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
	args = append(args, "-t", "0.6", "-c:v", codec, "-threads", "2", "-c:a", "aac", path)
	if output, err := exec.CommandContext(t.Context(), ffmpeg, args...).CombinedOutput(); err != nil {
		t.Fatalf("fixture: %v: %s", err, output)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
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
