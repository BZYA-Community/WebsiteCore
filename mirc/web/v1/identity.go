package v1

import (
	"github.com/BZYA-Community/WebsiteCore/internal/model/web"
	. "github.com/alimy/mir/v5"
)

// Identity exposes effective permissions without account details.
type Identity struct {
	Schema      `mir:"v1,chain"`
	GetIdentity func(Get, web.IdentityReq) web.IdentityResp `mir:"identity"`
}
