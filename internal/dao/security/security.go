package security

import (
	"github.com/BZYA-Community/WebsiteCore/internal/core"
)

func NewPhoneVerifyService() core.PhoneVerifyService {
	return newAliyunSmsServant()
}
