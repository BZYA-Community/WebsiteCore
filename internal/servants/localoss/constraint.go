//go:build constraint

package localoss

import (
	api "github.com/BZYA-Community/WebsiteCore/auto/api/s/v1"
)

var _ api.User = (*userSrv)(nil)
