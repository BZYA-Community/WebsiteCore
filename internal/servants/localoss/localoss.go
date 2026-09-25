// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package localoss

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	api "github.com/BZYA-Community/WebsiteCore/auto/api/s/v1"
	"github.com/BZYA-Community/WebsiteCore/internal/conf"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/storage"
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
	objectPath := path.Clean("/" + c.Param("filepath"))
	fullPath := filepath.Join(savePath, filepath.FromSlash(objectPath))

	// 防目录穿越: 解析后的真实路径必须仍位于savePath之内
	root, err := filepath.Abs(savePath)
	if err != nil || !strings.HasPrefix(fullPath, root+string(os.PathSeparator)) {
		c.String(http.StatusNotFound, "not found")
		return
	}
	fi, err := os.Stat(fullPath)
	if err != nil || fi.IsDir() {
		// 目录一律404，不提供列举
		c.String(http.StatusNotFound, "not found")
		return
	}
	// 下载附件类对象需校验签名链接(与storage.LocalOSSSign使用同一请求路径派生)
	reqPath := "/oss" + objectPath
	if strings.Contains(reqPath, "/attachment/") {
		expired, err := strconv.ParseInt(c.Query("expired"), 10, 64)
		if err != nil || !storage.VerifyLocalOSSSign(reqPath, expired, c.Query("sign")) {
			c.String(http.StatusForbidden, "invalid or expired signature")
			return
		}
	}
	c.File(fullPath)
}

// RouteLocaloss register LocalOSS route if needed
func RouteLocaloss(e *gin.Engine) {
	api.RegisterUserServant(e, newUserSrv())
}
