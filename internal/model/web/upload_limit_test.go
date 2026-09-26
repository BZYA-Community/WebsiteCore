// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package web

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// newUploadTestContext 构造带 multipart 请求体的测试上下文(#29)
func newUploadTestContext(t *testing.T, fieldValues map[string]string, fileName string, fileSize int) *gin.Context {
	t.Helper()

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for name, value := range fieldValues {
		if err := mw.WriteField(name, value); err != nil {
			t.Fatalf("WriteField(%q) error: %v", name, err)
		}
	}
	if fileName != "" {
		fw, err := mw.CreateFormFile(fileName, "file.bin")
		if err != nil {
			t.Fatalf("CreateFormFile error: %v", err)
		}
		if _, err := fw.Write(bytes.Repeat([]byte("a"), fileSize)); err != nil {
			t.Fatalf("write file part error: %v", err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close multipart writer error: %v", err)
	}

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	req := httptest.NewRequest(http.MethodPost, "/attachment", bytes.NewReader(buf.Bytes()))
	req.Header.Set("Content-Type", mw.FormDataContentType())
	c.Request = req
	return c
}

// TestApplyUploadBodyLimitRejectsOversizedBody 验证请求体超限在读取阶段被拒绝,
// 且映射为与 fileCheck 一致的"文件过大"错误(#29)
func TestApplyUploadBodyLimitRejectsOversizedBody(t *testing.T) {
	c := newUploadTestContext(t, map[string]string{"type": "attachment"}, "file", 4096)

	tooLarge := ErrFileInvalidSize.WithDetails("最大允许100MB")
	applyUploadBodyLimit(c, 1024) // 上限小于请求体

	err := parseUploadForm(c, tooLarge)
	if err == nil {
		t.Fatal("parseUploadForm() = nil, want body-too-large error")
	}
	if err != tooLarge {
		t.Fatalf("parseUploadForm() = %v, want sentinel %v", err, tooLarge)
	}
}

// TestApplyUploadBodyLimitAllowsNormalBody 验证限额内的请求正常解析(#29)
func TestApplyUploadBodyLimitAllowsNormalBody(t *testing.T) {
	c := newUploadTestContext(t, map[string]string{"type": "attachment"}, "file", 2048)

	applyUploadBodyLimit(c, maxUploadBodySize)
	if err := parseUploadForm(c, ErrFileInvalidSize.WithDetails("最大允许100MB")); err != nil {
		t.Fatalf("parseUploadForm() error = %v, want nil", err)
	}
	if got := c.Request.FormValue("type"); got != "attachment" {
		t.Fatalf("FormValue(type) = %q, want attachment", got)
	}
	file, fileHeader, err := c.Request.FormFile("file")
	if err != nil {
		t.Fatalf("FormFile() error = %v, want nil", err)
	}
	defer file.Close()
	if fileHeader.Size != 2048 {
		t.Fatalf("fileHeader.Size = %d, want 2048", fileHeader.Size)
	}
}

// TestParseUploadFormNonMultipart 验证非multipart请求按上传失败处理(与原FormFile路径一致)
func TestParseUploadFormNonMultipart(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/attachment", bytes.NewReader([]byte("type=attachment")))
	c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	applyUploadBodyLimit(c, maxUploadBodySize)
	if err := parseUploadForm(c, ErrFileInvalidSize.WithDetails("最大允许100MB")); err != ErrFileUploadFailed {
		t.Fatalf("parseUploadForm() = %v, want ErrFileUploadFailed", err)
	}
}

// TestUploadBodyLimitConstants 验证请求体上限留有开销余量且与文件上限语义一致(#29)
func TestUploadBodyLimitConstants(t *testing.T) {
	if maxUploadBodySize <= 100*1024*1024 {
		t.Fatalf("maxUploadBodySize = %d, want > 100MB (multipart开销余量)", maxUploadBodySize)
	}
	if maxCourseVideoBodySize <= 500*1024*1024 {
		t.Fatalf("maxCourseVideoBodySize = %d, want > 500MB (multipart开销余量)", maxCourseVideoBodySize)
	}
	if uploadFormMemory > maxUploadBodySize {
		t.Fatalf("uploadFormMemory = %d, want <= maxUploadBodySize", uploadFormMemory)
	}
}
