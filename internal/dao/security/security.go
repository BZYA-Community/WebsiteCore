package security

import (
	"strings"

	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/alimy/tryst/cfg"
)

func NewPhoneVerifyService() core.PhoneVerifyService {
	smsVendor, _ := cfg.Val("sms")
	switch strings.ToLower(smsVendor) {
	case "smsjuhe":
		return newJuheSmsServant()
	default:
		return newJuheSmsServant()
	}
}
