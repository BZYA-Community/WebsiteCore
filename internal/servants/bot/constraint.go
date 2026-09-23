//go:build constraint

package bot

import (
	api "github.com/BZYA-Community/WebsiteCore/auto/api/r/v1"
)

var _ api.User = (*userSrv)(nil)
