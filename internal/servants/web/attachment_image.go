package web

import (
	"image"
	_ "image/gif"
	"io"

	_ "github.com/anthonynsimon/bild/imgio"
	_ "golang.org/x/image/webp"
)

// attachmentImageConfig rewinds streams already consumed by object storage.
// Invalid image metadata remains non-fatal, preserving the upload contract.
func attachmentImageConfig(file io.ReadSeeker) (image.Config, error) {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return image.Config{}, err
	}
	config, _, err := image.DecodeConfig(file)
	if err != nil {
		return image.Config{}, nil
	}
	return config, nil
}
