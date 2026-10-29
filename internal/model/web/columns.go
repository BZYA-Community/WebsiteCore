package web

import (
	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/gin-gonic/gin"
)

type ColumnsReq struct {
	AuthorID     int64 `form:"author_id"`
	Index        int   `form:"index"`
	SubmissionID int64 `form:"submission_id"`
	BaseInfo     `form:"-" binding:"-"`
	ID           int64  `form:"id"`
	Kind         string `form:"kind"`
	Keyword      string `form:"keyword"`
	Mine         bool   `form:"mine"`
	Manage       bool   `form:"manage"`
	Official     bool   `form:"official"`
	Editing      bool   `form:"editing"`
	Returned     bool   `form:"returned"`
	Style        string `form:"style"`
	Page         int    `form:"-" binding:"-"`
	PageSize     int    `form:"-" binding:"-"`
}

func (r *ColumnsReq) SetPageInfo(page, size int) { r.Page, r.PageSize = page, size }

type ColumnsResp struct {
	Items []*ms.Column `json:"items"`
	Total int64        `json:"total"`
}
type ColumnResp struct {
	Item *ms.Column `json:"item"`
}
type ColumnCommentsResp struct {
	Items []*ms.ColumnComment `json:"items"`
	Total int64               `json:"total"`
}
type ColumnCommentResp struct {
	Item *ms.ColumnComment `json:"item"`
}
type ColumnReviewsResp struct {
	Items []*core.ColumnReviewItem `json:"items"`
	Total int64                    `json:"total"`
}

type ColumnSubmitReq struct {
	BaseInfo           `json:"-" binding:"-"`
	ID                 int64              `json:"id"`
	ExpectedSubmission int64              `json:"expected_submission"`
	RecreatedFromID    int64              `json:"recreated_from_id"`
	Kind               string             `json:"kind" binding:"required"`
	Title              string             `json:"title" binding:"required"`
	Summary            string             `json:"summary"`
	Body               string             `json:"body"`
	Cover              string             `json:"cover"`
	Attachments        []*PostContentItem `json:"attachments"`
}
type ColumnActionReq struct {
	BaseInfo `json:"-" binding:"-"`
	ID       int64  `json:"id" binding:"required"`
	Action   string `json:"action" binding:"required"`
	Reason   string `json:"reason"`
}
type ColumnEntriesReq struct {
	BaseInfo `json:"-" binding:"-"`
	ID       int64   `json:"id" binding:"required"`
	Articles []int64 `json:"articles"`
}
type ColumnCommentReq struct {
	BaseInfo `json:"-" binding:"-"`
	ColumnID int64              `json:"column_id" binding:"required"`
	ParentID int64              `json:"parent_id"`
	AtUserID int64              `json:"at_user_id"`
	Content  string             `json:"content"`
	Contents []*PostContentItem `json:"contents"`
	Users    []string           `json:"users"`
	ClientIP string             `json:"-" binding:"-"`
}

func (r *ColumnCommentReq) Bind(c *gin.Context) error {
	r.ClientIP = c.ClientIP()
	return bindAny(c, r)
}

type ColumnReviewReq struct {
	Kind     string `json:"kind" binding:"required"`
	TargetID int64  `json:"target_id" binding:"required"`
	BaseInfo `json:"-" binding:"-"`
	TaskID   int64  `json:"task_id" binding:"required"`
	Revision int64  `json:"revision" binding:"required"`
	Action   string `json:"action" binding:"required"`
	Reason   string `json:"reason"`
}
