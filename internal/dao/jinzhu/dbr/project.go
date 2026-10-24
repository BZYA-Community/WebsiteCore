package dbr

import "gorm.io/gorm"

type Project struct {
	*Model
	CreatorID   int64  `json:"creator_id"`
	PostID      int64  `json:"post_id"`
	Title       string `json:"title"`
	Intro       string `json:"intro"`
	URL         string `json:"url"`
	FollowCount int64  `json:"follow_count"`
}

type ProjectImage struct {
	*Model
	ProjectID int64  `json:"project_id"`
	URL       string `json:"url"`
	Sort      int64  `json:"sort"`
}

type ProjectTag struct {
	*Model
	ProjectID int64  `json:"project_id"`
	Name      string `json:"name"`
}

type ProjectMember struct {
	*Model
	ProjectID int64  `json:"project_id"`
	UserID    int64  `json:"user_id"`
	Role      string `json:"role"`
}

type ProjectFollow struct {
	ProjectID int64 `gorm:"primaryKey" json:"project_id"`
	UserID    int64 `gorm:"primaryKey" json:"user_id"`
	CreatedOn int64 `json:"created_on"`
}

type ProjectMemberFormated struct {
	User *UserFormated `json:"user"`
	Role string        `json:"role"`
}

type ProjectFormated struct {
	ID          int64                    `json:"id"`
	CreatorID   int64                    `json:"creator_id"`
	Creator     *UserFormated            `json:"creator"`
	PostID      int64                    `json:"post_id"`
	Title       string                   `json:"title"`
	Intro       string                   `json:"intro"`
	URL         string                   `json:"url"`
	Images      []string                 `json:"images"`
	Tags        []string                 `json:"tags"`
	Members     []*ProjectMemberFormated `json:"members"`
	FollowCount int64                    `json:"follow_count"`
	Followed    bool                     `json:"followed"`
	CreatedOn   int64                    `json:"created_on"`
}

func (p *Project) Create(db *gorm.DB) error { return db.Create(p).Error }
