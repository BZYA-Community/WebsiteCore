package xerror

import (
	"net/http"
	"testing"
)

func TestHttpStatusCodeAuthenticationAndPermission(t *testing.T) {
	for code, want := range map[int]int{20006: http.StatusUnauthorized, 20007: http.StatusForbidden} {
		status, gotCode := HttpStatusCode(NewError(code, "test"))
		if status != want || gotCode != code {
			t.Errorf("code %d: got HTTP %d and code %d, want HTTP %d", code, status, gotCode, want)
		}
	}
}
