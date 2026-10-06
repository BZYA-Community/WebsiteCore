package cache

import (
	"testing"

	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
)

func TestIndexCacheSeparatesCurrentVisibility(t *testing.T) {
	s := &cacheIndexSrv{}
	u := &ms.User{Model: &ms.Model{ID: 7}, Status: ms.UserStatusNormal, Permissions: []string{"post.view", "content.view_private"}}
	privileged := s.keyFrom(u, 0, 10)
	u.Permissions = []string{"post.view"}
	ordinary := s.keyFrom(u, 0, 10)
	if privileged == ordinary {
		t.Fatal("revoked private-content grant reused privileged cache")
	}
	u.Permissions = nil
	if ordinary == s.keyFrom(u, 0, 10) {
		t.Fatal("revoked post access reused visible cache")
	}
	if s.keyFrom(nil, 0, 10) == ordinary {
		t.Fatal("anonymous and user caches collided")
	}
}
