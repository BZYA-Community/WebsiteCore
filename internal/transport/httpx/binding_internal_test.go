package httpx

import (
	"errors"
	"fmt"
	"testing"

	"github.com/BZYA-Community/WebsiteCore/pkg/xerror"
	"github.com/go-playground/validator/v10"
)

// requiredField mirrors the minimal validated request shape used by the
// external tests in binding_test.go.
type requiredField struct {
	Name string `json:"name" form:"name" binding:"required"`
}

// newBindError must carry per-field validation details. The Sentry bind path
// historically built its error from an empty error list, losing the details
// that the plain path reported.
func TestNewBindErrorCarriesFieldDetails(t *testing.T) {
	validation := validator.New()
	var bindErrs validator.ValidationErrors
	if err := validation.Var("", "required"); !errors.As(err, &bindErrs) || len(bindErrs) == 0 {
		t.Fatalf("expected validation errors for empty required value, got %v", err)
	}

	mirErr := newBindError(bindErrs)
	if mirErr.StatusCode() != xerror.InvalidParams.StatusCode() {
		t.Fatalf("status = %d, want %d", mirErr.StatusCode(), xerror.InvalidParams.StatusCode())
	}

	details := newBindXError(bindErrs).Details()
	if len(details) != len(bindErrs) {
		t.Fatalf("details = %q, want one entry per failed field (%d)", details, len(bindErrs))
	}
	for i, fieldErr := range bindErrs {
		want := fmt.Sprintf("字段 %s 校验失败: %s", fieldErr.Field(), fieldErr.Tag())
		if details[i] != want {
			t.Fatalf("details[%d] = %q, want %q", i, details[i], want)
		}
	}
}

// A non-validation bind failure must still produce a detail entry, not an
// empty list.
func TestNewBindErrorWrapsRawBindFailure(t *testing.T) {
	details := newBindXError(errors.New("boom")).Details()
	if len(details) != 1 || details[0] != "boom" {
		t.Fatalf("details = %q, want [boom]", details)
	}
}
