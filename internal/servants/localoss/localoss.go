// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package localoss

import (
	"io"
	"mime"
	"net/http"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/BZYA-Community/WebsiteCore/internal/conf"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/storage"
	"github.com/BZYA-Community/WebsiteCore/internal/media"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// RouteLocalOSS register LocalOSS route if needed
func RouteLocalOSS(e *gin.Engine) {
	savePath, err := filepath.Abs(conf.LocalOSSSetting.SavePath)
	if err != nil {
		logrus.Fatalf("get localOSS save path err: %v", err)
	}
	// 安全修复: 弃用gin Static(其底层http.FileServer会输出目录列举)，
	// 改为自定义文件服务:
	//  1. 目录请求一律404，杜绝/oss目录列举泄露全部上传文件
	//  2. attachment/前缀的对象必须携带有效HMAC签名且未过期(仅经SignURL签发)
	e.GET("/oss/*filepath", func(c *gin.Context) {
		serveLocalOSSObject(c, savePath)
	})
	e.HEAD("/oss/*filepath", func(c *gin.Context) {
		serveLocalOSSObject(c, savePath)
	})

	logrus.Infof("register LocalOSS route in /oss on save path: %s", savePath)
}

// serveLocalOSSObject 安全地输出本地对象文件
func serveLocalOSSObject(c *gin.Context, savePath string) {
	c.Header("X-Content-Type-Options", "nosniff")
	rawPath := c.Param("filepath")
	// Windows resolves backslashes, case, and trailing dots/spaces differently
	// from URL paths. Reject aliases before enforcing private namespace rules.
	if strings.ContainsAny(rawPath, "\\:\x00") {
		c.String(http.StatusNotFound, "not found")
		return
	}
	objectPath := path.Clean("/" + rawPath)
	for _, component := range strings.Split(objectPath, "/") {
		if strings.HasPrefix(component, ".") || strings.EqualFold(component, "staging") || strings.TrimRight(component, " .") != component {
			c.String(http.StatusNotFound, "not found")
			return
		}
	}
	if conf.ObjectStorage != nil && conf.ObjectStorage.TempDir != "" && strings.Contains(strings.ToLower(objectPath), "/"+strings.ToLower(strings.Trim(conf.ObjectStorage.TempDir, "/"))+"/") {
		c.String(http.StatusNotFound, "not found")
		return
	}
	// 下载附件类对象需校验签名链接(与storage.LocalOSSSign使用同一请求路径派生)
	reqPath := "/oss" + objectPath
	// Only known public namespaces bypass signing. A private-path denylist is
	// insufficient on filesystems with alternate names such as NTFS 8.3 aliases.
	parts := strings.SplitN(strings.TrimPrefix(objectPath, "/"), "/", 3)
	privateObject := true
	if len(parts) == 3 {
		namespace := strings.ToLower(parts[1])
		privateObject = namespace != "public" && namespace != "image"
	}
	if privateObject {
		expired, err := strconv.ParseInt(c.Query("expired"), 10, 64)
		if err != nil || !storage.VerifyLocalOSSSign(reqPath, expired, c.Query("sign")) {
			c.String(http.StatusForbidden, "invalid or expired signature")
			return
		}
	}
	f, err := storage.OpenLocalObject(savePath, strings.TrimPrefix(objectPath, "/"))
	if err != nil {
		c.String(http.StatusNotFound, "not found")
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		c.String(http.StatusNotFound, "not found")
		return
	}
	// Sniff at most one header; do not trust an uploaded extension as a browser
	// content type. Non-video private resources download instead of executing.
	header, err := io.ReadAll(io.LimitReader(f, 512))
	if err != nil {
		c.String(http.StatusInternalServerError, "read failed")
		return
	}
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		c.String(http.StatusInternalServerError, "read failed")
		return
	}
	contentType := http.DetectContentType(header)
	if videoType, _, err := mime.ParseMediaType(mime.TypeByExtension(filepath.Ext(info.Name()))); err == nil && strings.HasPrefix(videoType, "video/") && media.ValidateHeader(header, videoType) == nil {
		contentType = videoType
	}
	c.Header("Content-Type", contentType)
	privateDownload := privateObject && !strings.HasPrefix(contentType, "video/")
	activeContent := !strings.HasPrefix(contentType, "image/") && !strings.HasPrefix(contentType, "video/") && !strings.HasPrefix(contentType, "audio/")
	if privateDownload || activeContent {
		c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": info.Name()}))
	}
	http.ServeContent(c.Writer, c.Request, info.Name(), info.ModTime(), f)
}
