// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package core

import (
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
)

// AuthorizationManageService 授权管理服务
type AuthorizationManageService interface {
	IsAllow(user *ms.User, action *ms.Action) bool
}
