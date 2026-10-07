package media

import (
	"bytes"
	"testing"
)

func TestUploadSignatureTypeAndSize(t *testing.T) {
	zip := []byte{'P', 'K', 3, 4, 0, 0, 0, 0}
	if kind, ext, err := ValidateFile(bytes.NewReader(zip), "课程.zip", "application/zip", int64(len(zip)), 15<<20); err != nil || kind != "application/zip" || ext != ".zip" {
		t.Fatal(kind, ext, err)
	}
	for _, test := range []struct {
		name, mime  string
		body        []byte
		size, limit int64
	}{
		{"bad.zip", "application/zip", []byte("<html>script</html>"), 18, 100},
		{"bad.exe", "application/zip", zip, 8, 100},
		{"../escape.zip", "application/zip", zip, 8, 100},
		{"empty.zip", "application/zip", nil, 0, 100},
		{"big.zip", "application/zip", zip, (15 << 20) + 1, 15 << 20},
		{"fake.png", "image/png", zip, 8, 100},
		{"active.svg", "image/svg+xml", []byte("<svg>"), 5, 100},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, _, err := ValidateFile(bytes.NewReader(test.body), test.name, test.mime, test.size, test.limit); err == nil {
				t.Fatal("invalid upload accepted")
			}
		})
	}
	webm := append([]byte{0x1a, 0x45, 0xdf, 0xa3}, []byte("\x42\x82\x84webm")...)
	if err := ValidateHeader(webm, "video/webm"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateHeader([]byte{0x1a, 0x45, 0xdf, 0xa3}, "video/webm"); err == nil {
		t.Fatal("generic EBML accepted as WebM")
	}
	video := []byte{0, 0, 0, 20, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm'}
	if err := ValidateHeader(video, "video/mp4"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateHeader(video, "video/quicktime"); err == nil {
		t.Fatal("MP4 accepted as QuickTime")
	}
	file := bytes.NewReader(zip)
	if _, _, err := ValidateFile(file, "valid.zip", "application/zip", 8, 8); err != nil {
		t.Fatal(err)
	}
	if pos, _ := file.Seek(0, 1); pos != 0 {
		t.Fatal("validation consumed upload stream")
	}
}
