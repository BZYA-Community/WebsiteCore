// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package space

import (
	api "github.com/BZYA-Community/WebsiteCore/auto/api/x/v1"
	"github.com/gin-gonic/gin"
)

// RouteWeb register SpaceX route
func RouteSpaceX(e *gin.Engine) {
	api.RegisterUserServant(e, newUserSrv())
}
