package media

import (
	"bytes"
	"context"
	"errors"
	"image"
	"io"
	"testing"

	"github.com/anthonynsimon/bild/imgio"
)

func TestValidateImage(t *testing.T) {
	for _, tc := range []struct {
		name string
		w, h int
		ok   bool
	}{
		{"small", 31, 17, true},
		{"landscape", 1920, 1080, true},
		{"portrait", 1080, 1920, true},
		{"wide", 1921, 1, false},
		{"tall", 1, 1921, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var data bytes.Buffer
			if err := imgio.WEBPEncoder(nil)(&data, image.NewNRGBA(image.Rect(0, 0, tc.w, tc.h))); err != nil {
				t.Fatal(err)
			}
			reader := bytes.NewReader(data.Bytes())
			if err := ValidateImage(t.Context(), reader, AttachmentLimit); (err == nil) != tc.ok {
				t.Fatalf("unexpected validation: %v", err)
			}
			if pos, _ := reader.Seek(0, io.SeekCurrent); pos != 0 {
				t.Fatal("validation must rewind file")
			}
			if err := ValidateImage(t.Context(), reader, int64(data.Len()-1)); !errors.Is(err, ErrTooLarge) {
				t.Fatal(err)
			}
			if err := ValidateImage(t.Context(), bytes.NewReader(data.Bytes()[:data.Len()/2]), AttachmentLimit); err == nil {
				t.Fatal("truncated image accepted")
			}
		})
	}
	for _, enc := range []imgio.Encoder{imgio.PNGEncoder(), imgio.JPEGEncoder(90)} {
		var data bytes.Buffer
		if err := enc(&data, image.NewNRGBA(image.Rect(0, 0, 10, 10))); err != nil {
			t.Fatal(err)
		}
		if err := ValidateImage(t.Context(), bytes.NewReader(data.Bytes()), AttachmentLimit); !errors.Is(err, ErrInvalid) {
			t.Fatal("unprocessed source accepted", err)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := ValidateImage(ctx, bytes.NewReader(nil), AttachmentLimit); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
