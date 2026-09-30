package content_test

import (
	"errors"
	"testing"

	"github.com/BZYA-Community/WebsiteCore/internal/application/content"
	"github.com/BZYA-Community/WebsiteCore/internal/core/cs"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
)

type memoryStore struct {
	post     *ms.Post
	contents []*ms.PostContent
	users    []*ms.User
	follows  map[int64]bool
	err      error
}

func (s *memoryStore) IsMyFollow(_ int64, _ ...int64) (map[int64]bool, error) {
	return s.follows, s.err
}
func (s *memoryStore) IsFollow(_, author int64) bool         { return s.follows[author] }
func (s *memoryStore) GetPostByID(_ int64) (*ms.Post, error) { return s.post, s.err }
func (s *memoryStore) GetPostContentsByIDs(_ []int64) ([]*ms.PostContent, error) {
	return s.contents, s.err
}
func (s *memoryStore) GetUsersByIDs(_ []int64) ([]*ms.User, error) { return s.users, s.err }
func (s *memoryStore) GetUserByUsername(name string) (*ms.User, error) {
	for _, user := range s.users {
		if user.Username == name {
			return user, s.err
		}
	}
	return nil, errors.New("missing user")
}
func user(id int64, roles string) *ms.User { return &ms.User{Model: &ms.Model{ID: id}, Roles: roles} }

func TestReadPolicyAcrossRepresentations(t *testing.T) {
	views := content.New(&memoryStore{follows: map[int64]bool{10: true}})
	for _, tc := range []struct {
		name       string
		viewer     *ms.User
		visibility ms.PostVisibleT
		audit      ms.PostAuditT
		want       bool
	}{
		{"public guest", nil, ms.PostVisitPublic, ms.PostAuditApproved, true},
		{"pending guest", nil, ms.PostVisitPublic, ms.PostAuditPending, false},
		{"pending author", user(10, ""), ms.PostVisitPrivate, ms.PostAuditPending, true},
		{"pending admin", &ms.User{Model: &ms.Model{ID: 20}, IsAdmin: true}, ms.PostVisitPrivate, ms.PostAuditPending, true},
		{"pending auditor", user(20, ms.RoleAuditor), ms.PostVisitPrivate, ms.PostAuditPending, true},
		{"mentor private", user(20, ms.RoleMentor), ms.PostVisitPrivate, ms.PostAuditApproved, false},
		{"following guest", nil, ms.PostVisitFollowing, ms.PostAuditApproved, false},
		{"following user", user(20, ""), ms.PostVisitFollowing, ms.PostAuditApproved, true},
		{"legacy friend", user(20, ""), ms.PostVisitFriend, ms.PostAuditApproved, false},
		{"unknown", user(20, ""), 99, ms.PostAuditApproved, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			post := &ms.Post{Model: &ms.Model{ID: 1}, UserID: 10, Visibility: tc.visibility, AuditStatus: tc.audit}
			for _, value := range []any{post, post.Format()} {
				if got := views.CanViewTweet(tc.viewer, value); got != tc.want {
					t.Fatalf("CanViewTweet(%T) = %v, want %v", value, got, tc.want)
				}
			}
		})
	}
	for _, post := range []any{nil, (*ms.Post)(nil), (*ms.PostFormated)(nil), "invalid"} {
		if views.CanViewTweet(user(10, ""), post) {
			t.Fatalf("invalid post %T accepted", post)
		}
	}
}

func TestReadAssemblesContentAndIsolatesDeletedAuthors(t *testing.T) {
	store := &memoryStore{
		post:     &ms.Post{Model: &ms.Model{ID: 5}, UserID: 10, Visibility: ms.PostVisitPublic, AuditStatus: ms.PostAuditApproved},
		contents: []*ms.PostContent{{Model: &ms.Model{ID: 8}, PostID: 5, Content: "hello", Type: ms.ContentTypeText}},
		follows:  map[int64]bool{10: true},
	}
	views := content.New(store)
	first, err := views.GetTweetBy(5)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Contents) != 1 || first.Contents[0].Content != "hello" || first.User.ID != 0 {
		t.Fatalf("view = %+v", first)
	}
	if err = views.PrepareTweet(user(20, ""), first); err != nil {
		t.Fatal(err)
	}
	second, err := views.GetTweetBy(5)
	if err != nil {
		t.Fatal(err)
	}
	if !first.User.IsFollowing || second.User.IsFollowing || ms.GhostUserFormated.IsFollowing {
		t.Fatal("viewer state leaked across responses")
	}
	store.users = []*ms.User{{Model: &ms.Model{ID: 10}, Username: "author", Nickname: "Author"}}
	third, err := views.GetTweetBy(5)
	if err != nil || third.User.Username != "author" {
		t.Fatalf("author view = %+v, %v", third, err)
	}
}

func TestEnrichmentReturnsStoreFailuresAndAllowsGuests(t *testing.T) {
	failure := errors.New("store unavailable")
	views := content.New(&memoryStore{err: failure})
	if _, err := views.GetTweetBy(5); !errors.Is(err, failure) {
		t.Fatalf("read error = %v", err)
	}
	people := []*ms.MessageFormated{{SenderUserID: 10, SenderUser: &ms.UserFormated{ID: 10}}}
	if err := views.PrepareMessages(20, people); !errors.Is(err, failure) {
		t.Fatalf("enrichment error = %v", err)
	}
	if err := views.PrepareMessages(-1, people); err != nil {
		t.Fatalf("guest enrichment = %v", err)
	}
}

func TestProfileRelationUsesAuthenticatedIdentity(t *testing.T) {
	self := &ms.User{Model: &ms.Model{ID: 10}, Username: "me"}
	views := content.New(&memoryStore{users: []*ms.User{self}})
	for _, tc := range []struct {
		viewer *ms.User
		name   string
		want   cs.RelationTyp
	}{
		{self, "me", cs.RelationSelf},
		{nil, "me", cs.RelationGuest},
		{&ms.User{Model: &ms.Model{ID: 20}, Username: "admin", IsAdmin: true}, "me", cs.RelationAdmin},
		{user(20, ""), "me", cs.RelationGuest},
	} {
		got, err := views.RelationTypFrom(tc.viewer, tc.name)
		if err != nil || got.UserId != 10 || got.RelTyp != tc.want {
			t.Fatalf("relation = %+v, %v", got, err)
		}
	}
	if _, err := views.RelationTypFrom(nil, "deleted"); err == nil {
		t.Fatal("missing profile accepted")
	}
}
