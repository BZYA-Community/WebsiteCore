package core

import "github.com/BZYA-Community/WebsiteCore/internal/core/ms"

type ColumnQuery struct {
	AuthorID               int64
	Kind, Keyword          string
	Mine, Manage, Official bool
	Offset, Limit          int
}

type ColumnReviewItem struct {
	Task       *ms.ReviewTask       `json:"task"`
	Column     *ms.Column           `json:"column"`
	Submission *ms.ColumnSubmission `json:"submission,omitempty"`
	Comment    *ms.ColumnComment    `json:"comment,omitempty"`
}

type ColumnService interface {
	CheckColumnMedia(actor *ms.User, id int64, contents []*ms.ColumnContent) error
	ColumnAttachment(actor *ms.User, id, submissionID int64, index int) (string, error)
	ListColumns(actor *ms.User, query ColumnQuery) ([]*ms.Column, int64, error)
	GetColumn(actor *ms.User, id int64, submission bool) (*ms.Column, error)
	SubmitColumn(actor *ms.User, id, expectedSubmission int64, kind string, submission *ms.ColumnSubmission) (*ms.Column, error)
	ColumnAction(actor *ms.User, id int64, action string) error
	ColumnEntries(actor *ms.User, id int64, editing bool) ([]*ms.Column, error)
	SetColumnEntries(actor *ms.User, id int64, articles []int64) error
	ColumnComments(actor *ms.User, id int64, style string, offset, limit int) ([]*ms.ColumnComment, int64, error)
	CreateColumnComment(actor *ms.User, comment *ms.ColumnComment) (*ms.ColumnComment, error)
	ColumnCommentAction(actor *ms.User, id int64, action string) error
	ListColumnReviews(actor *ms.User, offset, limit int) ([]*ColumnReviewItem, int64, error)
}
