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
	ListProjects(viewerID int64, viewerIsAuditor bool, keyword, tag string, offset, limit int) ([]*ms.ProjectFormated, int64, error)
	GetProject(viewerID int64, viewerIsAuditor bool, id int64) (*ms.ProjectFormated, error)
	GetProjectByPostID(postID int64) (*ms.Project, error)
	CreateProject(actor *ms.User, input *ProjectCreateInput) (*ms.Project, *ms.Post, error)
	SetProjectFollow(projectID, userID int64, follow bool) (bool, int64, error)
}
