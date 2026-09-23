//go:build constraint

package space

import (
	api "github.com/BZYA-Community/WebsiteCore/auto/api/x/v1"
)

var _ api.User = (*userSrv)(nil)
