// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package jinzhu

import (
	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"gorm.io/gorm"
)

type authorizationManageSrv struct {
	db *gorm.DB
}

func newAuthorizationManageService(db *gorm.DB) core.AuthorizationManageService {
	return &authorizationManageSrv{
		db: db,
	}
}

func (s *authorizationManageSrv) IsAllow(user *ms.User, action *ms.Action) bool {
	// user is activation if had bind phone
	isActivation := (len(user.Phone) != 0)
	// 好友功能已移除: isFriend恒为false
	return action.Act.IsAllow(user, action.UserId, false, isActivation)
}
