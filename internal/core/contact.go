package core

import "errors"

var (
	ErrContactInvalid     = errors.New("invalid contact verification request")
	ErrContactDisabled    = errors.New("contact verification channel is disabled or not configured")
	ErrContactRateLimited = errors.New("contact verification request limit reached")
	ErrContactCode        = errors.New("verification code is invalid, expired or exhausted")
	ErrContactInUse       = errors.New("contact address is already bound")
)
