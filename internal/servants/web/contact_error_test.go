package web

import (
	"fmt"
	"testing"

	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/pkg/xerror"
)

func TestContactAttemptLimitError(t *testing.T) {
	for _, err := range []error{core.ErrContactMaxAttempts, fmt.Errorf("wrapped: %w", core.ErrContactMaxAttempts)} {
		got, ok := contactError(err).(*xerror.Error)
		if !ok || got.StatusCode() != 23005 {
			t.Fatalf("attempt limit error = %v", got)
		}
	}
	got := contactError(core.ErrContactCode).(*xerror.Error)
	if got.StatusCode() != 23003 {
		t.Fatalf("invalid code error = %v", got)
	}
}
