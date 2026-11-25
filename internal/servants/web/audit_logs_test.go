package web

import (
	"reflect"
	"slices"
	"testing"

	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
)

type auditLogUserData struct {
	core.DataService
	operator *ms.User
	batches  [][]int64
}

func (d *auditLogUserData) GetUserByID(id int64) (*ms.User, error) {
	if id == d.operator.ID {
		return d.operator, nil
	}
	return nil, nil
}

func (d *auditLogUserData) GetUsersByIDs(ids []int64) ([]*ms.User, error) {
	d.batches = append(d.batches, slices.Clone(ids))
	return []*ms.User{d.operator}, nil
}

func TestAuditLogUsernamesUseOneBatch(t *testing.T) {
	data := &auditLogUserData{operator: &ms.User{Model: &ms.Model{ID: 3}, Username: "auditor"}}
	if names := usernamesOf(data, []int64{0, -1, 3, 4, 3}); !reflect.DeepEqual(names, map[int64]string{3: "auditor"}) {
		t.Fatalf("usernames = %v", names)
	}
	if !reflect.DeepEqual(data.batches, [][]int64{{3, 4}}) {
		t.Fatalf("username lookup batches = %v, want [[3 4]]", data.batches)
	}
}
