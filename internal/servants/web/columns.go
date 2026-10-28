package web

import (
	"strings"

	api "github.com/BZYA-Community/WebsiteCore/auto/api/v1"
	"github.com/BZYA-Community/WebsiteCore/internal/authz"
	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/internal/model/web"
	"github.com/BZYA-Community/WebsiteCore/internal/servants/base"
	"github.com/BZYA-Community/WebsiteCore/internal/servants/chain"
	"github.com/BZYA-Community/WebsiteCore/pkg/utils"
	"github.com/BZYA-Community/WebsiteCore/pkg/xerror"
	"github.com/gin-gonic/gin"
)

type columnsLooseSrv struct {
	api.UnimplementedColumnsLooseServant
	*base.DaoServant
	oss core.ObjectStorageService
}
type columnsPrivSrv struct {
	api.UnimplementedColumnsPrivServant
	*base.DaoServant
	oss core.ObjectStorageService
}

func (s *columnsLooseSrv) Chain() gin.HandlersChain {
	return gin.HandlersChain{chain.JwtLoose(), chain.Authorize()}
}
func (s *columnsPrivSrv) Chain() gin.HandlersChain {
	return gin.HandlersChain{chain.JWT(), chain.Authorize()}
}

func (s *columnsLooseSrv) Columns(r *web.ColumnsReq) (*web.ColumnsResp, error) {
	kind := r.Kind
	if kind == "" {
		kind = ms.ColumnArticle
	}
	rows, total, err := s.Ds.ListColumns(r.User, core.ColumnQuery{AuthorID: r.AuthorID, Kind: kind, Keyword: r.Keyword, Mine: r.Mine, Manage: r.Manage, Official: r.Official, Offset: (r.Page - 1) * r.PageSize, Limit: r.PageSize})
	return &web.ColumnsResp{Items: rows, Total: total}, reviewError(err)
}
func (s *columnsLooseSrv) Column(r *web.ColumnsReq) (*web.ColumnResp, error) {
	row, err := s.Ds.GetColumn(r.User, r.ID, false)
	return &web.ColumnResp{Item: row}, reviewError(err)
}
func (s *columnsPrivSrv) ColumnSubmission(r *web.ColumnsReq) (*web.ColumnResp, error) {
	row, err := s.Ds.GetColumn(r.User, r.ID, true)
	return &web.ColumnResp{Item: row}, reviewError(err)
}
func (s *columnsLooseSrv) ColumnEntries(r *web.ColumnsReq) (*web.ColumnsResp, error) {
	rows, err := s.Ds.ColumnEntries(r.User, r.ID, r.Editing)
	return &web.ColumnsResp{Items: rows, Total: int64(len(rows))}, reviewError(err)
}
func (s *columnsLooseSrv) ColumnComments(r *web.ColumnsReq) (*web.ColumnCommentsResp, error) {
	rows, total, err := s.Ds.ColumnComments(r.User, r.ID, r.Style, (r.Page-1)*r.PageSize, r.PageSize)
	return &web.ColumnCommentsResp{Items: rows, Total: total}, reviewError(err)
}

func (s *columnsPrivSrv) columnMedia(actor *ms.User, id int64, items []*web.PostContentItem) ([]*ms.ColumnContent, error) {
	contents := make([]*ms.ColumnContent, 0, len(items))
	for _, item := range items {
		if item == nil {
			return nil, xerror.InvalidParams
		}
		if err := item.Check(s.Ds); err != nil {
			return nil, xerror.InvalidParams
		}
		contents = append(contents, &ms.ColumnContent{Content: item.Content, Type: item.Type, Sort: item.Sort})
	}
	if err := s.Ds.CheckColumnMedia(actor, id, contents); err != nil {
		return nil, reviewError(err)
	}
	if err := persistMediaContents(s.oss, items); err != nil {
		return nil, xerror.ServerError
	}
	return contents, nil
}
func (s *columnsPrivSrv) SubmitColumn(r *web.ColumnSubmitReq) (*web.ColumnResp, error) {
	if r.User == nil || !r.User.HasPermission(authz.ColumnPublish) {
		return nil, web.ErrNoPermission
	}
	if r.ID > 0 {
		if _, err := s.Ds.GetColumn(r.User, r.ID, true); err != nil {
			return nil, reviewError(err)
		}
	}
	attachments, err := s.columnMedia(r.User, r.ID, r.Attachments)
	if err != nil {
		return nil, err
	}
	if r.Cover != "" {
		if !r.User.HasPermission(authz.ContentUpload) {
			return nil, web.ErrNoPermission
		}
		if _, err := s.columnMedia(r.User, r.ID, []*web.PostContentItem{{Content: r.Cover, Type: ms.ContentTypeImage, Sort: 1}}); err != nil {
			return nil, err
		}
	}
	if r.Kind == ms.ColumnArticle {
		summary := []rune(markdownSummaryText(r.Body))
		if len(summary) > 200 {
			summary = summary[:200]
		}
		r.Summary = string(summary)
	}
	row, err := s.Ds.SubmitColumn(r.User, r.ID, r.ExpectedSubmission, r.Kind, &ms.ColumnSubmission{Title: r.Title, Summary: r.Summary, Body: r.Body, Cover: r.Cover, Attachments: attachments})
	return &web.ColumnResp{Item: row}, reviewError(err)
}
func (s *columnsPrivSrv) ColumnAction(r *web.ColumnActionReq) error {
	return reviewError(s.Ds.ColumnAction(r.User, r.ID, r.Action))
}
func (s *columnsPrivSrv) SetColumnEntries(r *web.ColumnEntriesReq) error {
	return reviewError(s.Ds.SetColumnEntries(r.User, r.ID, r.Articles))
}
func (s *columnsPrivSrv) CreateColumnComment(r *web.ColumnCommentReq) (*web.ColumnCommentResp, error) {
	if r.User == nil || !r.User.HasPermission(authz.CommentCreate) {
		return nil, web.ErrNoPermission
	}
	content := r.Content
	if r.ParentID == 0 {
		texts := []string{}
		for _, item := range r.Contents {
			if item != nil && item.Type == ms.ContentTypeText {
				texts = append(texts, item.Content)
			}
		}
		content = strings.Join(texts, "\n")
	}
	c := &ms.ColumnComment{ColumnID: r.ColumnID, ParentID: r.ParentID, AtUserID: r.AtUserID, Content: content, Mentions: r.Users, IPLoc: utils.GetIPLoc(r.ClientIP)}
	for _, item := range r.Contents {
		if item != nil {
			c.Contents = append(c.Contents, &ms.ColumnContent{Content: item.Content, Type: item.Type, Sort: item.Sort})
		}
	}
	for _, item := range c.Contents {
		if item.Type == ms.ContentTypeImage && !r.User.HasPermission(authz.ContentUpload) {
			return nil, web.ErrNoPermission
		}
	}
	// Validate all references before committing a comment containing them.
	if _, err := s.columnMedia(r.User, 0, r.Contents); err != nil {
		return nil, err
	}
	row, err := s.Ds.CreateColumnComment(r.User, c)
	return &web.ColumnCommentResp{Item: row}, reviewError(err)
}
func (s *columnsPrivSrv) ColumnCommentAction(r *web.ColumnActionReq) error {
	return reviewError(s.Ds.ColumnCommentAction(r.User, r.ID, r.Action))
}
func (s *columnsPrivSrv) ColumnReviews(r *web.ColumnsReq) (*web.ColumnReviewsResp, error) {
	rows, total, err := s.Ds.ListColumnReviews(r.User, (r.Page-1)*r.PageSize, r.PageSize)
	return &web.ColumnReviewsResp{Items: rows, Total: total}, reviewError(err)
}
func (s *columnsPrivSrv) DecideColumnReview(r *web.ColumnReviewReq) error {
	if r.Kind != ms.ReviewColumn && r.Kind != ms.ReviewColumnComment {
		return xerror.InvalidParams
	}
	task, err := s.Ds.ReviewTaskForTarget(r.User, r.Kind, r.TargetID)
	if err != nil {
		return reviewError(err)
	}
	if task.ID != r.TaskID {
		return web.ErrNoPermission
	}
	_, err = s.Ds.DecideReview(r.User, r.TaskID, r.Revision, r.Action, r.Reason)
	return reviewError(err)
}

func (s *columnsLooseSrv) ColumnAttachment(r *web.ColumnsReq) (*web.DownloadAttachmentResp, error) {
	content, err := s.Ds.ColumnAttachment(r.User, r.ID, r.SubmissionID, r.Index)
	if err != nil {
		return nil, reviewError(err)
	}
	key := s.oss.ObjectKey(content)
	if key == "" {
		return nil, xerror.InvalidParams
	}
	signed, err := s.oss.SignURL(key, 600)
	if err != nil {
		return nil, xerror.ServerError
	}
	return &web.DownloadAttachmentResp{SignedURL: signed}, nil
}
