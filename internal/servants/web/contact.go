package web

import (
	"errors"
	"strings"

	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/model/web"
	"github.com/BZYA-Community/WebsiteCore/pkg/xerror"
)

func contactError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, core.ErrContactInvalid):
		return xerror.InvalidParams
	case errors.Is(err, core.ErrContactDisabled):
		return xerror.NewError(23001, "This verification channel is unavailable")
	case errors.Is(err, core.ErrContactRateLimited):
		return xerror.NewError(23002, "Too many verification requests; try again later")
	case errors.Is(err, core.ErrContactCode):
		return xerror.NewError(23003, "Invalid or expired verification code")
	case errors.Is(err, core.ErrContactInUse):
		return xerror.NewError(23004, "This contact address is already bound")
	default:
		return xerror.ServerError
	}
}

func (s *coreSrv) SendContactCode(req *web.SendContactCodeReq) error {
	return contactError(s.Ds.SendContactCode(req.Context, req.User.ID, req.Mode, req.Address, req.ClientIP))
}

func (s *coreSrv) VerifyContactCode(req *web.VerifyContactCodeReq) error {
	if err := s.Ds.VerifyContactCode(req.Context, req.User.ID, req.Mode, req.Address, req.Code); err != nil {
		return contactError(err)
	}
	onChangeUsernameEvent(req.User.ID, req.User.Username)
	return nil
}

func maskEmail(address string) string {
	local, domain, ok := strings.Cut(address, "@")
	if !ok || local == "" {
		return ""
	}
	return string([]rune(local)[0]) + "***@" + domain
}
