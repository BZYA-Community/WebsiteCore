package web

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/BZYA-Community/WebsiteCore/internal/application/content"
	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	model "github.com/BZYA-Community/WebsiteCore/internal/model/web"
	"github.com/BZYA-Community/WebsiteCore/internal/servants/base"
)

type starsViewData struct {
	core.DataService
	post          *ms.Post
	followErr     error
	viewer        int64
	limit, offset int
}

func (d *starsViewData) GetUserPostStars(viewer int64, limit, offset int) ([]*ms.PostStar, error) {
	d.viewer, d.limit, d.offset = viewer, limit, offset
	return []*ms.PostStar{{Post: d.post}}, nil
}
func (d *starsViewData) GetUserPostStarsWithCount(viewer int64, limit, offset int) ([]*ms.PostStar, int64, error) {
	stars, err := d.GetUserPostStars(viewer, limit, offset)
	return stars, 3, err
}
func (*starsViewData) GetUserPostStarCount(int64) (int64, error) { return 3, nil }
func (*starsViewData) MergePosts(posts []*ms.Post) ([]*ms.PostFormated, error) {
	return []*ms.PostFormated{posts[0].Format()}, nil
}
func (d *starsViewData) IsMyFollow(int64, ...int64) (map[int64]bool, error) {
	return map[int64]bool{d.post.UserID: true}, d.followErr
}

func TestGetStarsUsesPublicPostView(t *testing.T) {
	d := &starsViewData{post: &ms.Post{Model: &ms.Model{ID: 12}, UserID: 7, Visibility: ms.PostVisitPublic, AuditStatus: ms.PostAuditApproved}}
	s := &coreSrv{DaoServant: &base.DaoServant{Ds: d, Views: content.New(d)}}
	resp, err := s.GetStars(&model.GetStarsReq{UserId: 9, Page: 2, PageSize: 1})
	if err != nil {
		t.Fatal(err)
	}
	// Check the public JSON contract, not just the persistence enum.
	encoded, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	var actual struct {
		List []struct {
			ID         int64 `json:"id"`
			Visibility int   `json:"visibility"`
			User       struct {
				IsFollowing bool `json:"is_following"`
			} `json:"user"`
		} `json:"list"`
	}
	if err := json.Unmarshal(encoded, &actual); err != nil {
		t.Fatal(err)
	}
	if len(actual.List) != 1 || actual.List[0].ID != 12 || actual.List[0].Visibility != 0 || !actual.List[0].User.IsFollowing {
		t.Fatalf("unexpected public stars response: %s", encoded)
	}
	if d.viewer != 9 || d.limit != 1 || d.offset != 1 || resp.Pager.TotalRows != 3 || resp.Pager.Page != 2 {
		t.Fatalf("pagination changed: %#v", resp.Pager)
	}

	d.followErr = errors.New("follow metadata unavailable")
	if result, err := s.GetStars(&model.GetStarsReq{UserId: 9, Page: 1, PageSize: 1}); result != nil || err != model.ErrGetStarsFailed {
		t.Fatalf("view assembly failure returned result=%v, err=%v", result, err)
	}
}
