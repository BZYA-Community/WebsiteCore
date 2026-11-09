package web

import (
	"errors"
	"strings"
	"testing"

	"github.com/BZYA-Community/WebsiteCore/internal/authz"
	"github.com/BZYA-Community/WebsiteCore/internal/conf"
	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	model "github.com/BZYA-Community/WebsiteCore/internal/model/web"
	"github.com/BZYA-Community/WebsiteCore/internal/servants/base"
	"gorm.io/gorm"
)

type projectCreateData struct {
	core.DataService
	getProjectCalled bool
}

func (*projectCreateData) CheckAttachment(string) error { return nil }

func (*projectCreateData) CreateProject(_ *ms.User, input *core.ProjectCreateInput) (*ms.Project, *ms.Post, error) {
	input.Project.Model = &ms.Model{ID: 11, CreatedOn: 123}
	input.Project.PostID = 12
	input.Post.Model = &ms.Model{ID: 12}
	input.Post.AuditStatus = ms.PostAuditPending
	return input.Project, input.Post, nil
}

func (d *projectCreateData) GetProject(*ms.User, int64) (*ms.ProjectFormated, error) {
	d.getProjectCalled = true
	return nil, errors.New("GetProject must not be called after creation")
}

type projectDetailData struct {
	core.DataService
	err error
}

func (d *projectDetailData) GetProject(*ms.User, int64) (*ms.ProjectFormated, error) {
	return nil, d.err
}

func TestCreateTweetRejectsProjectWhenDisabled(t *testing.T) {
	previous := conf.ProjectSetting
	t.Cleanup(func() { conf.ProjectSetting = previous })
	conf.ProjectSetting = &conf.ProjectsConf{Enabled: false}

	req := &model.CreateTweetReq{
		BaseInfo: model.BaseInfo{User: permissionUser(1, "post.create")},
		Tags:     []string{" 项目 "},
		Contents: []*model.PostContentItem{{Content: "Project", Type: ms.ContentTypeText, Sort: 1}},
	}
	if _, err := (&privSrv{}).CreateTweet(req); err != model.ErrProjectsDisabled {
		t.Fatalf("CreateTweet() error = %v, want %v", err, model.ErrProjectsDisabled)
	}
}

func TestProjectTagDetection(t *testing.T) {
	for _, test := range []struct {
		name string
		tags []string
		want bool
	}{
		{name: "exact", tags: []string{"项目"}, want: true},
		{name: "trimmed", tags: []string{"news", " 项目 "}, want: true},
		{name: "different tag", tags: []string{"projects"}, want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := hasProjectPostTag(test.tags); got != test.want {
				t.Fatalf("hasProjectPostTag(%q) = %t, want %t", test.tags, got, test.want)
			}
		})
	}
}

func TestCreateProjectDoesNotRequireViewPermissionAfterCommit(t *testing.T) {
	previousProjects, previousEvent := conf.ProjectSetting, onProjectCreated
	t.Cleanup(func() {
		conf.ProjectSetting = previousProjects
		onProjectCreated = previousEvent
	})
	conf.ProjectSetting = &conf.ProjectsConf{Enabled: true}
	onProjectCreated = func(*ms.User) {}

	creator := permissionUser(7, "post.create", "content.upload")
	creator.Username = "creator"
	if creator.HasPermission("post.view") {
		t.Fatal("test creator unexpectedly has post.view")
	}
	data := &projectCreateData{}
	servant := &privSrv{
		DaoServant: &base.DaoServant{Ds: data},
		oss:        &sharedAttachmentStorage{},
	}
	response, err := servant.CreateProject(&model.CreateProjectReq{
		BaseInfo: model.BaseInfo{User: creator},
		Title:    "Project",
		Intro:    "Intro",
		URL:      "https://example.com",
		Images:   []string{sharedAttachmentURL},
		Tags:     []string{"Go"},
	})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	if data.getProjectCalled {
		t.Fatal("CreateProject() called GetProject after committing")
	}

	if response.ID != 11 || response.PostID != 12 || response.Creator.ID != creator.ID {
		t.Fatalf("response identity = project %d post %d creator %d", response.ID, response.PostID, response.Creator.ID)
	}
	if len(response.Images) != 1 || response.Images[0] != sharedAttachmentURL || len(response.Tags) != 1 || response.Tags[0] != "Go" {
		t.Fatalf("response media/tags = %#v / %#v", response.Images, response.Tags)
	}
	if len(response.Members) != 1 || response.Members[0].Role != "owner" {
		t.Fatalf("response members = %#v", response.Members)
	}
}

func TestProjectDetailClassifiesStoreErrors(t *testing.T) {
	previous := conf.ProjectSetting
	t.Cleanup(func() { conf.ProjectSetting = previous })
	conf.ProjectSetting = &conf.ProjectsConf{Enabled: true}

	for _, test := range []struct {
		name string
		err  error
		want error
	}{
		{name: "not found", err: gorm.ErrRecordNotFound, want: model.ErrProjectNotExist},
		{name: "store failure", err: errors.New("database unavailable"), want: model.ErrGetProjectsFailed},
	} {
		t.Run(test.name, func(t *testing.T) {
			servant := &looseSrv{DaoServant: &base.DaoServant{Ds: &projectDetailData{err: test.err}}}
			if _, err := servant.ProjectDetail(&model.ProjectDetailReq{ID: 7}); err != test.want {
				t.Fatalf("ProjectDetail() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestCreateProjectRejectsURLBeyondInputLimit(t *testing.T) {
	previous := conf.ProjectSetting
	t.Cleanup(func() { conf.ProjectSetting = previous })
	conf.ProjectSetting = &conf.ProjectsConf{Enabled: true}

	_, err := (&privSrv{}).CreateProject(&model.CreateProjectReq{
		BaseInfo: model.BaseInfo{User: permissionUser(7, "post.create", "content.upload")},
		Title:    "Project",
		Intro:    "Intro",
		URL:      "https://example.com/" + strings.Repeat("a", projectURLMaxLength),
		Images:   []string{sharedAttachmentURL},
	})
	status, ok := err.(interface{ StatusCode() int })
	if !ok || status.StatusCode() != model.ErrProjectInputInvalid.StatusCode() {
		t.Fatalf("CreateProject() error = %v, want project input error", err)
	}
}

type projectUpdateData struct {
	core.DataService
	input    *core.ProjectUpdateInput
	err      error
	response *ms.ProjectFormated
}

func (*projectUpdateData) CheckAttachment(string) error { return nil }

func (d *projectUpdateData) UpdateProject(_ *ms.User, input *core.ProjectUpdateInput) (*ms.ProjectFormated, error) {
	d.input = input
	if d.err != nil {
		return nil, d.err
	}
	if err := input.PersistMedia(); err != nil {
		return nil, err
	}
	input.Post = &ms.Post{Model: &ms.Model{ID: d.response.PostID}}
	return d.response, nil
}

func TestUpdateProjectUsesCommittedStateAndPreservesErrors(t *testing.T) {
	oldProjects, oldEvent := conf.ProjectSetting, onProjectUpdated
	t.Cleanup(func() { conf.ProjectSetting, onProjectUpdated = oldProjects, oldEvent })
	conf.ProjectSetting = &conf.ProjectsConf{Enabled: true}
	creator := permissionUser(7, "post.create", "content.upload")
	events := 0
	onProjectUpdated = func(*privSrv, *ms.Post) { events++ }
	for _, test := range []struct {
		name string
		err  error
		want error
	}{
		{"success", nil, nil},
		{"not owner", authz.ErrDenied, model.ErrNoPermission},
		{"not found", gorm.ErrRecordNotFound, model.ErrProjectNotExist},
		{"database unavailable", errors.New("offline"), model.ErrUpdateProjectFailed},
	} {
		t.Run(test.name, func(t *testing.T) {
			data := &projectUpdateData{err: test.err, response: &ms.ProjectFormated{
				ID: 11, PostID: 12, CreatorID: creator.ID, Followed: true, FollowCount: 5,
				Members: []*ms.ProjectMemberFormated{{Role: "owner"}, {Role: "member"}},
			}}
			storage := &sharedAttachmentStorage{}
			before := events
			response, err := (&privSrv{DaoServant: &base.DaoServant{Ds: data}, oss: storage}).UpdateProject(&model.UpdateProjectReq{
				BaseInfo: model.BaseInfo{User: creator}, ID: 11, Title: " Project ", Intro: " Intro ", URL: "https://example.com",
				Images: []string{sharedAttachmentURL, "https://images.example.test/project.png"}, Tags: []string{" Go ", "Go", ""},
			})
			if err != test.want {
				t.Fatalf("UpdateProject() error = %v, want %v", err, test.want)
			}
			if test.err != nil {
				if response != nil || len(storage.persisted) != 0 || events != before {
					t.Fatalf("failed update had side effects: storage=%v events=%d", storage.persisted, events-before)
				}
				return
			}
			if response != (*model.UpdateProjectResp)(data.response) || !response.Followed || len(response.Members) != 2 || events-before != 1 {
				t.Fatalf("lost committed state: %+v", response)
			}
			if len(storage.persisted) != 1 || data.input.Project.Title != "Project" || data.input.Project.Intro != "Intro" || len(data.input.Tags) != 1 || data.input.Tags[0].Name != "Go" {
				t.Fatalf("input not normalized/persisted: %+v / %v", data.input, storage.persisted)
			}
		})
	}
}

func TestUpdateProjectRejectsMissingPermissions(t *testing.T) {
	oldProjects := conf.ProjectSetting
	t.Cleanup(func() { conf.ProjectSetting = oldProjects })
	conf.ProjectSetting = &conf.ProjectsConf{Enabled: true}
	for _, user := range []*ms.User{nil, permissionUser(7, "post.create"), permissionUser(7, "content.upload")} {
		_, err := (&privSrv{}).UpdateProject(&model.UpdateProjectReq{ID: 11, BaseInfo: model.BaseInfo{User: user}})
		if err != model.ErrNoPermission {
			t.Fatalf("permissions=%+v error=%v", user, err)
		}
	}
}

func TestCreateProjectSupportsExternalImageWithoutStorage(t *testing.T) {
	oldProjects, oldEvent := conf.ProjectSetting, onProjectCreated
	t.Cleanup(func() { conf.ProjectSetting, onProjectCreated = oldProjects, oldEvent })
	conf.ProjectSetting = &conf.ProjectsConf{Enabled: true}
	onProjectCreated = func(*ms.User) {}
	storage := &sharedAttachmentStorage{}
	response, err := (&privSrv{DaoServant: &base.DaoServant{Ds: &projectCreateData{}}, oss: storage}).CreateProject(&model.CreateProjectReq{
		BaseInfo: model.BaseInfo{User: permissionUser(7, "post.create", "content.upload")},
		Title:    "Project", Intro: "Intro", URL: "https://example.com", Images: []string{"https://images.example.test/project.png"},
	})
	if err != nil || len(storage.persisted) != 0 || len(response.Images) != 1 || response.Images[0] != "https://images.example.test/project.png" {
		t.Fatalf("CreateProject() response=%+v error=%v storage=%v", response, err, storage.persisted)
	}
}
