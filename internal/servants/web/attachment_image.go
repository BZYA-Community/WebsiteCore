package web

import (
	"image"
	_ "image/gif" // bild/imgio does not register GIF.
	"io"

	_ "github.com/anthonynsimon/bild/imgio" // Register JPEG, PNG and WebP decoders.
)

// attachmentImageSize only decodes the header. Object storage may have consumed
// the stream already; metadata must not depend on its current position. A failed
// probe keeps the existing upload behavior: the attachment has unknown dimensions.
func attachmentImageSize(file io.ReadSeeker) (int, int) {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return 0, 0
	}
	config, _, err := image.DecodeConfig(file)
	if err != nil {
		return 0, 0
	}
	return config.Width, config.Height
}
