//go:build constraint

package admin

import (
	api "github.com/BZYA-Community/WebsiteCore/auto/api/m/v1"
)

var _ api.User = (*userSrv)(nil)
