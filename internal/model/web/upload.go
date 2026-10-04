package web

import (
	"archive/zip"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

var uploadSlots = make(chan struct{}, 12)

// Bound the entire multipart body before FormValue/FormFile can spill it to
// disk. Cleanup belongs to the servant so disk-backed files live until storage
// has consumed them; failed binding must invoke it immediately.
func parseUpload(c *gin.Context, fileLimit int64) (func(), error) {
	select {
	case uploadSlots <- struct{}{}:
	default:
		return nil, ErrFileUploadFailed.WithDetails("上传繁忙，请稍后重试")
	}
	const uploadTimeout = 15 * time.Minute
	deadline := time.Now().Add(uploadTimeout + time.Minute)
	ctx, cancel := context.WithDeadline(c.Request.Context(), deadline)
	c.Request = c.Request.WithContext(ctx)
	controller := http.NewResponseController(c.Writer)
	_ = controller.SetReadDeadline(time.Now().Add(uploadTimeout))
	_ = controller.SetWriteDeadline(deadline)
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, fileLimit+(1<<20))
	cleanup := func() {
		cancel()
		if c.Request.MultipartForm != nil {
			_ = c.Request.MultipartForm.RemoveAll()
		}
		<-uploadSlots
	}
	if err := c.Request.ParseMultipartForm(8 << 20); err != nil {
		cleanup()
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return nil, ErrFileInvalidSize
		}
		return nil, ErrFileUploadFailed
	}
	if len(c.Request.MultipartForm.File["file"]) != 1 || len(c.Request.MultipartForm.File) != 1 {
		cleanup()
		return nil, ErrFileUploadFailed.WithDetails("每次请求只允许上传一个文件")
	}
	return cleanup, nil
}

// File extensions and MIME headers are untrusted. Sniff the file and, for ZIP,
// read its directory without extracting any entries.
func uploadFileFormat(file multipart.File, size int64, kind string) (string, string, error) {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", "", ErrFileInvalidExt
	}
	var header [512]byte
	n, err := file.Read(header[:])
	if err != nil && !errors.Is(err, io.EOF) {
		return "", "", ErrFileInvalidExt
	}
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		return "", "", ErrFileInvalidExt
	}
	contentType := http.DetectContentType(header[:n])
	switch kind {
	case "public/image", "public/avatar", "public/course-image":
		if contentType == "image/webp" {
			return ".webp", contentType, nil
		}
		return "", "", ErrFileInvalidExt.WithDetails("请使用浏览器将 PNG、JPEG 图片转换为 WebP 后上传")
	case "public/video", "course/video":
		if contentType == "video/mp4" {
			return ".mp4", contentType, nil
		}
		return "", "", ErrFileInvalidExt.WithDetails("视频仅允许 MP4 格式")
	case "attachment":
		if _, err := zip.NewReader(file, size); err == nil {
			return ".zip", "application/zip", nil
		}
		return "", "", ErrFileInvalidExt.WithDetails("附件仅允许 ZIP 格式")
	default:
		return "", "", ErrFileInvalidExt
	}
}
