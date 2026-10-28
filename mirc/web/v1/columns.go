package v1

import (
	"github.com/BZYA-Community/WebsiteCore/internal/model/web"
	. "github.com/alimy/mir/v5"
)

type ColumnsLoose struct {
	ColumnAttachment func(Get, web.ColumnsReq) web.DownloadAttachmentResp `mir:"column/attachment"`
	Schema           `mir:"v1,chain"`
	Columns          func(Get, web.ColumnsReq) web.ColumnsResp        `mir:"columns"`
	Column           func(Get, web.ColumnsReq) web.ColumnResp         `mir:"column"`
	ColumnEntries    func(Get, web.ColumnsReq) web.ColumnsResp        `mir:"column/entries"`
	ColumnComments   func(Get, web.ColumnsReq) web.ColumnCommentsResp `mir:"column/comments"`
}

type ColumnsPriv struct {
	Schema              `mir:"v1,chain"`
	ColumnSubmission    func(Get, web.ColumnsReq) web.ColumnResp               `mir:"column/submission"`
	SubmitColumn        func(Post, web.ColumnSubmitReq) web.ColumnResp         `mir:"column/submit"`
	ColumnAction        func(Post, web.ColumnActionReq)                        `mir:"column/action"`
	SetColumnEntries    func(Post, web.ColumnEntriesReq)                       `mir:"column/entries"`
	CreateColumnComment func(Post, web.ColumnCommentReq) web.ColumnCommentResp `mir:"column/comment"`
	ColumnCommentAction func(Post, web.ColumnActionReq)                        `mir:"column/comment/action"`
	ColumnReviews       func(Get, web.ColumnsReq) web.ColumnReviewsResp        `mir:"admin/audit/columns"`
	DecideColumnReview  func(Post, web.ColumnReviewReq)                        `mir:"admin/audit/column"`
}
