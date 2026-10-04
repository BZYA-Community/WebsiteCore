package web

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/jinzhu/dbr"
	model "github.com/BZYA-Community/WebsiteCore/internal/model/web"
	"github.com/BZYA-Community/WebsiteCore/internal/servants/base"
	"github.com/anthonynsimon/bild/imgio"
)

func attachmentImageFixtures(t *testing.T) map[string][]byte {
	t.Helper()
	source := image.NewRGBA(image.Rect(0, 0, 31, 17))
	source.Set(2, 3, color.RGBA{R: 255, A: 255})
	encoders := map[string]imgio.Encoder{
		"jpeg": imgio.JPEGEncoder(90),
		"png":  imgio.PNGEncoder(),
		"webp": imgio.WEBPEncoder(nil),
		"gif": func(w io.Writer, img image.Image) error {
			return gif.Encode(w, img, nil)
		},
	}
	fixtures := make(map[string][]byte, len(encoders))
	for format, encode := range encoders {
		var data bytes.Buffer
		if err := encode(&data, source); err != nil {
			t.Fatal(err)
		}
		fixtures[format] = data.Bytes()
	}
	return fixtures
}

func TestAttachmentImageSizeAfterStorageRead(t *testing.T) {
	for format, data := range attachmentImageFixtures(t) {
		t.Run(format, func(t *testing.T) {
			file := bytes.NewReader(data)
			if _, err := io.Copy(io.Discard, file); err != nil {
				t.Fatal(err)
			}
			width, height := attachmentImageSize(file)
			if width != 31 || height != 17 {
				t.Fatalf("dimensions = %dx%d; want 31x17", width, height)
			}
		})
	}
}

type unseekableImage struct{ *bytes.Reader }

func (unseekableImage) Seek(int64, int) (int64, error) {
	return 0, errors.New("seek failed")
}

func TestAttachmentImageSizeFailures(t *testing.T) {
	for name, file := range map[string]io.ReadSeeker{
		"empty":   bytes.NewReader(nil),
		"invalid": bytes.NewReader([]byte("not an image")),
		"seek":    unseekableImage{bytes.NewReader(nil)},
	} {
		t.Run(name, func(t *testing.T) {
			width, height := attachmentImageSize(file)
			if width != 0 || height != 0 {
				t.Fatalf("invalid image dimensions = %dx%d; want unknown (0x0)", width, height)
			}
		})
	}
}

type attachmentRecordingStorage struct {
	core.ObjectStorageService
	stored    []byte
	key, mime string
	size      int64
	deleted   bool
	err       error
}

func (s *attachmentRecordingStorage) PutObject(key string, reader io.Reader, size int64, mime string, _ bool) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	s.key, s.mime, s.size = key, mime, size
	var err error
	s.stored, err = io.ReadAll(reader)
	if closer, ok := reader.(io.Closer); ok {
		_ = closer.Close()
	}
	return "https://storage.example/public/image/test", err
}

func (s *attachmentRecordingStorage) ObjectKey(string) string   { return s.key }
func (s *attachmentRecordingStorage) DeleteObject(string) error { s.deleted = true; return nil }

type attachmentRecordingData struct {
	core.DataService
	attachment *ms.Attachment
	err        error
}

func (d *attachmentRecordingData) CreateAttachment(attachment *ms.Attachment) (int64, error) {
	attachment.Model = &dbr.Model{ID: 1}
	d.attachment = attachment
	return 1, d.err
}

func TestUploadAttachmentStoresProcessedImage(t *testing.T) {
	fixtures := attachmentImageFixtures(t)
	fixtures["invalid"] = []byte("not an image")
	for format, data := range fixtures {
		t.Run(format, func(t *testing.T) {
			// Multipart uploads can be disk-backed. Storage adapters may call Close;
			// this must not close the underlying file before dimensions are read.
			file, err := os.CreateTemp(t.TempDir(), "upload-*")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = file.Close() })
			if _, err := file.Write(data); err != nil {
				t.Fatal(err)
			}
			if _, err := file.Seek(0, io.SeekStart); err != nil {
				t.Fatal(err)
			}
			storage := &attachmentRecordingStorage{}
			database := &attachmentRecordingData{}
			srv := &privSrv{DaoServant: &base.DaoServant{Ds: database}, oss: storage}
			response, err := srv.UploadAttachment(&model.UploadAttachmentReq{
				SimpleInfo: model.SimpleInfo{Uid: 1},
				UploadType: "public/image", ContentType: "image/" + format,
				File: file, FileSize: int64(len(data)), FileExt: "." + format,
			})
			if format != "webp" {
				if err == nil || storage.stored != nil || database.attachment != nil {
					t.Fatal("unsupported image reached storage")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(storage.stored, data) {
				t.Fatal("server changed browser output")
			}
			width, height := 31, 17
			if response.ImgWidth != width || response.ImgHeight != height ||
				database.attachment.ImgWidth != width || database.attachment.ImgHeight != height {
				t.Fatalf("stored or returned dimensions differ from %dx%d", width, height)
			}
			if !strings.HasSuffix(storage.key, ".webp") || storage.mime != "image/webp" || response.FileSize != int64(len(storage.stored)) || storage.size != response.FileSize {
				t.Fatal("stored key, MIME or size does not describe normalized WebP")
			}
		})
	}
}
