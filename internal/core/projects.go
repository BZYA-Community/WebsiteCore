package core

import "github.com/BZYA-Community/WebsiteCore/internal/core/ms"

type ProjectCreateInput struct {
	Project  *ms.Project
	Images   []*ms.ProjectImage
	Tags     []*ms.ProjectTag
	Post     *ms.Post
	Contents []*ms.PostContent
}

type ProjectService interface {
	ListProjects(viewer *ms.User, keyword, tag string, followed bool, offset, limit int) ([]*ms.ProjectFormated, int64, error)
	GetProject(viewer *ms.User, id int64) (*ms.ProjectFormated, error)
	GetAuditProjects(actor *ms.User, postIDs []int64) (map[int64]*ms.ProjectFormated, error)
	CreateProject(actor *ms.User, input *ProjectCreateInput) (*ms.Project, *ms.Post, error)
	SetProjectFollow(projectID, userID int64, follow bool) (bool, int64, error)
}
