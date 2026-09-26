// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package servants

import (
	"github.com/BZYA-Community/WebsiteCore/internal/servants/docs"
	"github.com/BZYA-Community/WebsiteCore/internal/servants/localoss"
	"github.com/BZYA-Community/WebsiteCore/internal/servants/statick"
	"github.com/BZYA-Community/WebsiteCore/internal/servants/web"
	"github.com/alimy/tryst/cfg"
	"github.com/gin-gonic/gin"
)

// RegisterWebServants register all the servants to gin.Engine
func RegisterWebServants(e *gin.Engine) {
	cfg.Be("Frontend:EmbedWeb", func() {
		statick.RegisterWebStatick(e)
	})
	cfg.Be("LocalOSS", func() {
		localoss.RouteLocalOSS(e)
	})
	web.RouteWeb(e)
}

// RegisterDocsServants register all the servants to gin.Engine
func RegisterDocsServants(e *gin.Engine) {
	docs.RegisterDocs(e)
}

// RegisterFrontendWebServants register all the servants to gin.Engine
func RegisterFrontendWebServants(e *gin.Engine) {
	statick.RegisterWebStatick(e)
}
