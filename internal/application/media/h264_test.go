package media

import (
	"bytes"
	"testing"
)

func TestH264SPSInvalidAndTruncated(t *testing.T) {
	sps := fixtureSPS(t, videoFixture(t, "1920x1080", "30000/1001", "libx264", false))
	w, h, err := h264Dimensions(sps)
	if err != nil || w != 1920 || h != 1080 {
		t.Fatalf("cropped SPS: %dx%d, %v", w, h, err)
	}
	for _, bad := range [][]byte{
		nil, sps[:4], append([]byte{0xe7}, sps[1:]...),
		append([]byte{0x68}, sps[1:]...), bytes.Repeat([]byte{0}, 65536),
		{0x67, 66, 0, 30, 0, 0, 3}, // truncated emulation-prevention byte
		{0x67, 66, 0, 30, 0, 0, 3, 4},
		append([]byte{0x67, 66, 0, 30}, bytes.Repeat([]byte{0}, 8)...),
	} {
		if _, _, err := h264Dimensions(bad); err == nil {
			t.Fatalf("accepted malformed SPS: %x", bad[:min(8, len(bad))])
		}
	}
}

func FuzzH264Dimensions(f *testing.F) {
	f.Add([]byte{0x67, 66, 0, 30, 0xff})
	f.Add([]byte{0x67, 100, 0, 31, 0xac, 0xd9, 0x40})
	f.Fuzz(func(t *testing.T, data []byte) {
		w, h, err := h264Dimensions(data)
		if err == nil && (w == 0 || h == 0 || max(w, h) > 1920 || min(w, h) > 1080) {
			t.Fatalf("invalid accepted dimensions %dx%d", w, h)
		}
	})
}
