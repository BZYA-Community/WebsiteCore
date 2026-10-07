// Package media validates declared type, signature and size without executing or transcoding uploads.
package media

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path"
	"strings"
)

var extensions = map[string][]string{
	"image/jpeg": {".jpg", ".jpeg"}, "image/png": {".png"}, "image/gif": {".gif"}, "image/webp": {".webp"},
	"video/mp4": {".mp4"}, "video/quicktime": {".mov"}, "video/webm": {".webm"},
	"audio/mpeg": {".mp3"}, "audio/wav": {".wav"}, "audio/ogg": {".ogg"},
	"application/pdf": {".pdf"}, "application/zip": {".zip"},
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document":   {".docx"},
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         {".xlsx"},
	"application/vnd.openxmlformats-officedocument.presentationml.presentation": {".pptx"},
	"text/plain": {".txt", ".md"}, "text/csv": {".csv"},
}

func TypeAndExtension(name, declared string) (string, string, error) {
	if name == "" || len(name) > 255 || strings.ContainsAny(name, "/\\\x00\r\n") {
		return "", "", fmt.Errorf("invalid filename")
	}
	mediaType, _, err := mime.ParseMediaType(declared)
	if err != nil {
		return "", "", fmt.Errorf("invalid MIME type")
	}
	if mediaType == "image/jpg" {
		mediaType = "image/jpeg"
	}
	if mediaType == "audio/wave" || mediaType == "audio/x-wav" {
		mediaType = "audio/wav"
	}
	if mediaType == "application/ogg" {
		mediaType = "audio/ogg"
	}
	if mediaType == "application/x-zip-compressed" || mediaType == "application/x-zip" {
		mediaType = "application/zip"
	}
	ext := strings.ToLower(path.Ext(name))
	for _, allowed := range extensions[mediaType] {
		if ext == allowed {
			return mediaType, ext, nil
		}
	}
	return "", "", fmt.Errorf("unsupported or mismatched file extension and MIME type")
}

func ValidateHeader(header []byte, declared string) error {
	if len(header) == 0 {
		return fmt.Errorf("empty upload")
	}
	detected, _, err := mime.ParseMediaType(http.DetectContentType(header))
	if err != nil {
		return err
	}
	valid := detected == declared
	switch declared {
	case "video/mp4", "video/quicktime":
		valid = len(header) >= 12 && bytes.Equal(header[4:8], []byte("ftyp"))
		if valid {
			brand := string(header[8:12])
			valid = (declared == "video/quicktime" && brand == "qt  ") || (declared == "video/mp4" && brand != "qt  ")
		}
	case "video/webm":
		valid = bytes.HasPrefix(header, []byte{0x1a, 0x45, 0xdf, 0xa3}) && bytes.Contains(header, []byte("webm"))
	case "audio/mpeg":
		valid = bytes.HasPrefix(header, []byte("ID3")) || (len(header) > 1 && header[0] == 0xff && header[1]&0xe0 == 0xe0)
	case "audio/wav":
		valid = detected == "audio/wave" || detected == "audio/x-wav" || detected == "audio/wav"
	case "audio/ogg":
		valid = detected == "application/ogg" || detected == "audio/ogg"
	case "application/zip", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "application/vnd.openxmlformats-officedocument.presentationml.presentation":
		valid = bytes.HasPrefix(header, []byte{'P', 'K', 3, 4}) || bytes.HasPrefix(header, []byte{'P', 'K', 5, 6}) || bytes.HasPrefix(header, []byte{'P', 'K', 7, 8})
	case "text/csv":
		valid = detected == "text/plain"
	}
	if !valid {
		return fmt.Errorf("file signature does not match MIME type")
	}
	return nil
}

func ValidateFile(file io.ReadSeeker, name, declared string, size, limit int64) (string, string, error) {
	if size <= 0 || size > limit {
		return "", "", fmt.Errorf("file size is outside the upload limit")
	}
	mediaType, ext, err := TypeAndExtension(name, declared)
	if err != nil {
		return "", "", err
	}
	header := make([]byte, 512)
	n, err := file.Read(header)
	if err != nil && err != io.EOF {
		return "", "", err
	}
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		return "", "", err
	}
	if err = ValidateHeader(header[:n], mediaType); err != nil {
		return "", "", err
	}
	return mediaType, ext, nil
}
