package web

import (
	"testing"

	"github.com/BZYA-Community/WebsiteCore/internal/conf"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	model "github.com/BZYA-Community/WebsiteCore/internal/model/web"
)

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
