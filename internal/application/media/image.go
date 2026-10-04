// Package media validates browser-processed uploads before object storage.
package media

import (
	"context"
	"errors"
	"image"
	"io"

	_ "github.com/anthonynsimon/bild/imgio" // Registers JPEG, PNG and WebP decoders.
)

const (
	AttachmentLimit  int64 = 5 << 20
	VideoLimit       int64 = 30 << 20
	CourseImageLimit int64 = 60 << 20
	CourseVideoLimit int64 = 1 << 30
	ImageMaxEdge           = 1920
)

var (
	ErrInvalid  = errors.New("invalid processed media")
	ErrTooLarge = errors.New("media exceeds size limit")
)

// checkSize always rewinds the stream, including after callers have consumed it.
func checkSize(source io.ReadSeeker, limit int64) (int64, error) {
	size, err := source.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, err
	}
	if _, err = source.Seek(0, io.SeekStart); err != nil {
		return 0, err
	}
	if size > limit {
		return 0, ErrTooLarge
	}
	if size <= 0 {
		return 0, ErrInvalid
	}
	return size, nil
}

// ValidateImage accepts only the WebP produced by the browser. DecodeConfig
// bounds allocations before full decoding catches corrupt/truncated payloads.
// Conversion, resizing and EXIF orientation are performed by the client.
func ValidateImage(ctx context.Context, source io.ReadSeeker, limit int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := checkSize(source, limit); err != nil {
		return err
	}
	defer func() { _, _ = source.Seek(0, io.SeekStart) }()
	c, format, err := image.DecodeConfig(source)
	if err != nil || format != "webp" || c.Width <= 0 || c.Height <= 0 || c.Width > ImageMaxEdge || c.Height > ImageMaxEdge {
		return ErrInvalid
	}
	if _, err = source.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if _, _, err = image.Decode(source); err != nil {
		return ErrInvalid
	}
	return ctx.Err()
}
