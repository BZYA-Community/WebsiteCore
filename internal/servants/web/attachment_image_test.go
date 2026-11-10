package web

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"testing"
)

func TestAttachmentImageConfig(t *testing.T) {
	fixture := image.NewRGBA(image.Rect(0, 0, 3, 2))
	encoders := map[string]func(io.Writer) error{
		"jpeg": func(w io.Writer) error { return jpeg.Encode(w, fixture, nil) },
		"png":  func(w io.Writer) error { return png.Encode(w, fixture) },
		"gif":  func(w io.Writer) error { return gif.Encode(w, fixture, nil) },
	}
	for name, encode := range encoders {
		t.Run(name, func(t *testing.T) {
			var data bytes.Buffer
			if err := encode(&data); err != nil {
				t.Fatal(err)
			}
			for _, consumed := range []bool{false, true} {
				reader := bytes.NewReader(data.Bytes())
				if consumed {
					if _, err := io.Copy(io.Discard, reader); err != nil {
						t.Fatal(err)
					}
				}
				config, err := attachmentImageConfig(reader)
				if err != nil || config.Width != 3 || config.Height != 2 {
					t.Fatalf("consumed=%v: config=%+v error=%v", consumed, config, err)
				}
			}
		})
	}
	// A 1x1 lossless WebP, kept inline to avoid binary fixture dependencies.
	webp, err := base64.StdEncoding.DecodeString("UklGRhoAAABXRUJQVlA4TA0AAAAvAAAAEAcQERGIiP4HAA==")
	if err != nil {
		t.Fatal(err)
	}
	reader := bytes.NewReader(webp)
	if _, err := reader.Seek(0, io.SeekEnd); err != nil {
		t.Fatal(err)
	}
	config, err := attachmentImageConfig(reader)
	if err != nil || config.Width != 1 || config.Height != 1 {
		t.Fatalf("webp: config=%+v error=%v", config, err)
	}
	for _, data := range [][]byte{nil, []byte("not an image"), {0x89, 'P', 'N', 'G'}} {
		config, err := attachmentImageConfig(bytes.NewReader(data))
		if err != nil || config.Width != 0 || config.Height != 0 {
			t.Fatalf("invalid image should remain non-fatal: %+v, %v", config, err)
		}
	}
}

type failingImageSeeker struct{ *bytes.Reader }

func (failingImageSeeker) Seek(int64, int) (int64, error) {
	return 0, errors.New("seek failed")
}

func TestAttachmentImageConfigSeekFailure(t *testing.T) {
	if _, err := attachmentImageConfig(failingImageSeeker{bytes.NewReader(nil)}); err == nil {
		t.Fatal("ignored seek failure")
	}
}
