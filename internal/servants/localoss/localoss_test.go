package localoss

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/BZYA-Community/WebsiteCore/internal/conf"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/storage"
	"github.com/gin-gonic/gin"
)

func TestLocalDownloadAuthorizationAndHeaders(t *testing.T) {
	oldJWT, oldStorage := conf.JWTSetting, conf.ObjectStorage
	conf.JWTSetting, conf.ObjectStorage = nil, nil
	if err := json.Unmarshal([]byte(`{"Secret":"local-test-key-only"}`), &conf.JWTSetting); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(`{"TempDir":"tmp"}`), &conf.ObjectStorage); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conf.JWTSetting, conf.ObjectStorage = oldJWT, oldStorage })
	root := t.TempDir()
	files := map[string][]byte{
		"bucket/attachment/course/resources/document.pdf": []byte("%PDF-1.7\nfixture"),
		"bucket/attachment/course/resources/video.mp4":    {0, 0, 0, 24, 'f', 't', 'y', 'p', 'm', 'p', '4', '2', 0, 0, 0, 0, 'm', 'p', '4', '2', 'i', 's', 'o', 'm'},
		"bucket/attachment/course/staging/unverified.pdf": []byte("%PDF-unverified"),
		"bucket/tmp/public/image/unverified.png":          []byte("unverified"),
		"bucket/public/image/spoofed.png":                 []byte("<!DOCTYPE html><script>bad()</script>"),
		"bucket/ATTACH~1/course/resources/document.pdf":   []byte("%PDF-private-alias"),
		"bucket/unknown/document.pdf":                     []byte("%PDF-unclassified"),
	}
	for name, data := range files {
		full := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(full), 0750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	request := func(method, name string, signed bool, expired int64, byteRange string) *httptest.ResponseRecorder {
		t.Helper()
		requestPath := "/oss/" + name
		uri := (&url.URL{Path: requestPath}).String()
		if signed {
			uri += fmt.Sprintf("?expired=%d&sign=%s", expired, storage.LocalOSSSign(requestPath, expired))
		}
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(method, uri, nil)
		c.Request.Header.Set("Range", byteRange)
		c.Params = gin.Params{{Key: "filepath", Value: "/" + name}}
		serveLocalOSSObject(c, root)
		if w.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Fatal("missing nosniff")
		}
		return w
	}
	const document = "bucket/attachment/course/resources/document.pdf"
	expires := time.Now().Add(time.Minute).Unix()
	if w := request(http.MethodGet, document, false, expires, ""); w.Code != http.StatusForbidden {
		t.Fatalf("unsigned=%d", w.Code)
	}
	for _, name := range []string{"bucket/ATTACH~1/course/resources/document.pdf", "bucket/unknown/document.pdf"} {
		if w := request(http.MethodGet, name, false, expires, ""); w.Code != http.StatusForbidden {
			t.Fatalf("unclassified private namespace %q readable: %d", name, w.Code)
		}
	}
	for _, name := range []string{"bucket/Attachment/course/resources/document.pdf", "BUCKET/ATTACHMENT/course/resources/document.pdf", `bucket\attachment\course\resources\document.pdf`, "bucket/attachment./course/resources/document.pdf", "bucket/attachment /course/resources/document.pdf"} {
		if w := request(http.MethodGet, name, false, expires, ""); w.Code != http.StatusForbidden && w.Code != http.StatusNotFound {
			t.Fatalf("private Windows path alias %q readable: %d", name, w.Code)
		}
	}
	if w := request(http.MethodGet, document, true, time.Now().Add(-time.Minute).Unix(), ""); w.Code != http.StatusForbidden {
		t.Fatalf("expired=%d", w.Code)
	}
	if w := request(http.MethodGet, document, true, expires, ""); w.Code != http.StatusOK || w.Header().Get("Content-Type") != "application/pdf" || !strings.HasPrefix(w.Header().Get("Content-Disposition"), "attachment") {
		t.Fatalf("download status=%d headers=%v", w.Code, w.Header())
	}
	if w := request(http.MethodHead, document, true, expires, ""); w.Code != http.StatusOK || w.Body.Len() != 0 {
		t.Fatalf("HEAD=%d,%d", w.Code, w.Body.Len())
	}
	if w := request(http.MethodGet, document, true, expires, "bytes=0-3"); w.Code != http.StatusPartialContent || w.Body.String() != "%PDF" {
		t.Fatalf("Range=%d,%q", w.Code, w.Body.String())
	}
	if w := request(http.MethodGet, "bucket/attachment/course/resources/video.mp4", true, expires, ""); w.Code != http.StatusOK || w.Header().Get("Content-Type") != "video/mp4" || w.Header().Get("Content-Disposition") != "" {
		t.Fatalf("video headers=%v status=%d", w.Header(), w.Code)
	}
	for _, name := range []string{"bucket/attachment/course/staging/unverified.pdf", "bucket/attachment/course/STAGING/unverified.pdf", "bucket/tmp/public/image/unverified.png", "bucket/TMP/public/image/unverified.png", "bucket/public/image", "bucket/.upload-temp"} {
		if w := request(http.MethodGet, name, true, expires, ""); w.Code != http.StatusNotFound {
			t.Fatalf("internal object %s readable: %d", name, w.Code)
		}
	}
	if w := request(http.MethodGet, "bucket/public/image/spoofed.png", false, expires, ""); !strings.HasPrefix(w.Header().Get("Content-Disposition"), "attachment") {
		t.Fatal("active content rendered inline")
	}
	conf.JWTSetting = nil
	if storage.VerifyLocalOSSSign("/oss/x", expires, storage.LocalOSSSign("/oss/x", expires)) {
		t.Fatal("empty signing key accepted")
	}
}
