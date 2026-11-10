package web

import (
	"reflect"
	"testing"

	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	model "github.com/BZYA-Community/WebsiteCore/internal/model/web"
	"github.com/BZYA-Community/WebsiteCore/internal/servants/base"
)

type editingAuditProjectData struct {
	core.DataService
	calls    []string
	revision int64
}

func (d *editingAuditProjectData) ListAuditPosts(*ms.User, int, int, int) ([]*ms.Post, int64, error) {
	return []*ms.Post{{Model: &ms.Model{ID: 12}}}, 1, nil
}

func (d *editingAuditProjectData) ReviewTaskForTarget(*ms.User, string, int64) (*ms.ReviewTask, error) {
	d.calls = append(d.calls, "revision")
	return &ms.ReviewTask{ID: 3, Revision: d.revision}, nil
}

func (d *editingAuditProjectData) MergePosts([]*ms.Post) ([]*ms.PostFormated, error) {
	d.calls = append(d.calls, "post body")
	return []*ms.PostFormated{{ID: 12}}, nil
}

func (d *editingAuditProjectData) GetAuditProjects(*ms.User, []int64) (map[int64]*ms.ProjectFormated, error) {
	d.calls = append(d.calls, "project body")
	projects := map[int64]*ms.ProjectFormated{12: {ID: 11, Title: "Old project body"}}
	// Simulate an owner committing an edit immediately after this body was
	// read. The response must not attach the new revision to this old body.
	d.revision++
	return projects, nil
}

func TestAuditProjectsCaptureRevisionBeforeMutableContent(t *testing.T) {
	data := &editingAuditProjectData{revision: 1}
	response, err := (&auditSrv{DaoServant: &base.DaoServant{Ds: data}}).ListAuditPosts(&model.AdminAuditPostsReq{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(data.calls, []string{"revision", "post body", "project body"}) {
		t.Fatalf("unsafe review read order: %v", data.calls)
	}
	posts := response.List.([]*ms.PostFormated)
	if len(posts) != 1 || posts[0].ReviewTask.Revision >= data.revision || posts[0].Project.Title != "Old project body" {
		t.Fatalf("old body paired with approvable new revision: %+v current revision=%d", posts, data.revision)
	}
}
