package core

import "github.com/BZYA-Community/WebsiteCore/internal/core/ms"

type ProjectCreateInput struct {
	Project  *ms.Project
	Images   []*ms.ProjectImage
	Tags     []*ms.ProjectTag
	Post     *ms.Post
	Contents []*ms.PostContent
}

type ProjectUpdateInput struct {
	Project *ms.Project
	Images  []*ms.ProjectImage
	Tags    []*ms.ProjectTag
	// PersistMedia runs only after transactional permission and ownership checks.
	PersistMedia func() error
	// Post is the committed linked post, for cache and search synchronization.
	Post *ms.Post
}

type ProjectService interface {
	ListProjects(viewer *ms.User, keyword, tag string, followed bool, offset, limit int) ([]*ms.ProjectFormated, int64, error)
	GetProject(viewer *ms.User, id int64) (*ms.ProjectFormated, error)
	GetAuditProjects(actor *ms.User, postIDs []int64) (map[int64]*ms.ProjectFormated, error)
	CreateProject(actor *ms.User, input *ProjectCreateInput) (*ms.Project, *ms.Post, error)
	UpdateProject(actor *ms.User, input *ProjectUpdateInput) (*ms.ProjectFormated, error)
	SetProjectFollow(projectID, userID int64, follow bool) (bool, int64, error)
}
