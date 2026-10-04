package web

import (
	"archive/zip"
	"bytes"
	"image"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"net/textproto"
	"testing"

	"github.com/anthonynsimon/bild/imgio"

	"github.com/BZYA-Community/WebsiteCore/internal/application/media"
	"github.com/gin-gonic/gin"
)

func uploadContextForTest(t *testing.T, kind, mime string, data []byte) *gin.Context {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	if err := w.WriteField("type", kind); err != nil {
		t.Fatal(err)
	}
	if err := w.WriteField("course_id", "42"); err != nil {
		t.Fatal(err)
	}
	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition", `form-data; name="file"; filename="untrusted.png"`)
	header.Set("Content-Type", mime)
	part, err := w.CreatePart(header)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = part.Write(data); err != nil {
		t.Fatal(err)
	}
	if err = w.Close(); err != nil {
		t.Fatal(err)
	}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/upload", &body)
	c.Request.Header.Set("Content-Type", w.FormDataContentType())
	c.Set("UID", int64(1))
	return c
}

func TestAttachmentBindingInspectsFileContents(t *testing.T) {
	var webpData, zipData bytes.Buffer
	if err := imgio.WEBPEncoder(nil)(&webpData, image.NewNRGBA(image.Rect(0, 0, 4, 3))); err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(&zipData)
	part, err := z.Create("example.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = io.WriteString(part, "example"); err != nil {
		t.Fatal(err)
	}
	if err = z.Close(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, kind, mime, wantMime string
		data                       []byte
		ok                         bool
	}{
		{"webp despite false header", "public/image", "text/plain", "image/webp", webpData.Bytes(), true},
		{"avatar", "public/avatar", "image/webp", "image/webp", webpData.Bytes(), true},
		{"course image", "public/course-image", "image/webp", "image/webp", webpData.Bytes(), true},
		{"spoofed webp", "public/image", "image/webp", "", []byte("<html>bad</html>"), false},
		{"image in video channel", "public/video", "video/mp4", "", webpData.Bytes(), false},
		{"real zip", "attachment", "application/octet-stream", "application/zip", zipData.Bytes(), true},
		{"fake zip", "attachment", "application/zip", "", []byte("not a zip"), false},
		{"unknown type", "public/other", "image/webp", "", webpData.Bytes(), false},
		{"empty", "public/image", "image/webp", "", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := uploadContextForTest(t, tc.kind, tc.mime, tc.data)
			var req UploadAttachmentReq
			err := req.Bind(c)
			if (err == nil) != tc.ok {
				t.Fatalf("unexpected bind result: %v", err)
			}
			if err == nil {
				if req.ContentType != tc.wantMime || req.CourseID != 42 {
					t.Fatalf("bad metadata: %s %d", req.ContentType, req.CourseID)
				}
				_ = req.File.Close()
				req.Cleanup()
			}
			if len(uploadSlots) != 0 {
				t.Fatal("upload slot leaked")
			}
		})
	}
}

func TestMultipartBodyBoundAndCleanup(t *testing.T) {
	c := uploadContextForTest(t, "public/image", "image/webp", make([]byte, (1<<20)+4096))
	cleanup, err := parseUpload(c, 1024)
	if err == nil {
		cleanup()
		t.Fatal("oversized multipart body accepted")
	}
	if len(uploadSlots) != 0 {
		t.Fatal("upload slot leaked after oversized body")
	}
	if fileCheck("attachment", media.AttachmentLimit) != nil || fileCheck("attachment", media.AttachmentLimit+1) == nil {
		t.Fatal("ZIP boundary not enforced")
	}
}

func TestUploadSizeBoundaries(t *testing.T) {
	for kind, limit := range map[string]int64{
		"public/image": media.AttachmentLimit, "public/avatar": media.AttachmentLimit,
		"public/video": media.VideoLimit, "public/course-image": media.CourseImageLimit, "attachment": media.AttachmentLimit,
	} {
		if fileCheck(kind, limit) != nil || fileCheck(kind, limit+1) == nil || fileCheck(kind, 0) == nil {
			t.Fatalf("incorrect file limit for %s", kind)
		}
	}
}
