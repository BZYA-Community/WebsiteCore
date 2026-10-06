package search

import (
	"testing"

	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
)

func TestSearchFiltersUsePermissions(t *testing.T) {
	for _, tt := range []struct {
		name        string
		permissions []string
		want        int
	}{
		{"revoked", nil, 0},
		{"legacy admin", []string{"post.view"}, 1},
		{"reviewer", []string{"post.view", "content.review"}, 1},
		{"private viewer", []string{"post.view", "content.view_private"}, 2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			user := &ms.User{Model: &ms.Model{ID: 1}, Status: ms.UserStatusNormal, Roles: "admin", IsAdmin: true, Permissions: tt.permissions}
			resp := &core.QueryResp{Total: 2, Items: []*ms.PostFormated{{ID: 3, UserID: 2, Visibility: core.PostVisitPrivate}, {ID: 4, UserID: 2, Visibility: core.PostVisitPublic}}}
			(&tweetSearchFilter{}).filterResp(user, resp)
			if len(resp.Items) != tt.want || int(resp.Total) != tt.want {
				t.Fatalf("hits=%d total=%d, want %d", len(resp.Items), resp.Total, tt.want)
			}
			filter := (&meiliTweetSearchServant{publicFilter: "visibility=90"}).filterList(user)
			if (filter == "") != (tt.want == 2) {
				t.Fatalf("unexpected privileged filter: %q", filter)
			}
		})
	}
}
