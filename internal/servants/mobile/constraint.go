//go:build constraint

package mobile

import (
	api "github.com/BZYA-Community/WebsiteCore/auto/rpc/greet/v1"
)

var _ api.GreetServiceServer = (*greetServiceSrv)(nil)
