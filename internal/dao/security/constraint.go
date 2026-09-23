//go:build constraint

package security

import (
	"github.com/BZYA-Community/WebsiteCore/internal/core"
)

var (
	_ core.AttachmentCheckService = (*attachmentCheckServant)(nil)
	_ core.PhoneVerifyService     = (*juheSmsServant)(nil)
)
