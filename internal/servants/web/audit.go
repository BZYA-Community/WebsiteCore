// Copyright 2023 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package web

import (
	"github.com/gin-gonic/gin"
	api "github.com/BZYA-Community/WebsiteCore/auto/api/v1"
	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/cache"
	"github.com/BZYA-Community/WebsiteCore/internal/model/web"
	"github.com/BZYA-Community/WebsiteCore/internal/servants/base"
	"github.com/BZYA-Community/WebsiteCore/internal/servants/chain"
	"github.com/BZYA-Community/WebsiteCore/pkg/xerror"
	"github.com/sirupsen/logrus"
)

type auditSrv struct {
	api.UnimplementedAuditServant
	*base.DaoServant
	oss core.ObjectStorageService
}

func (s *auditSrv) Chain() gin.HandlersChain {
	return gin.HandlersChain{chain.JWT(), chain.Auditor()}
}

// ListAuditPosts 审核队列
func (s *auditSrv) ListAuditPosts(req *web.AdminAuditPostsReq) (*web.AdminAuditPostsResp, error) {
	limit, offset := req.PageSize, (req.Page-1)*req.PageSize
	posts, total, err := s.Ds.ListAuditPosts(req.Status, offset, limit)
	if err != nil {
		logrus.Errorf("Ds.ListAuditPosts err: %s", err)
		return nil, web.ErrGetPostsFailed
	}
	formated, err := s.Ds.MergePosts(posts)
	if err != nil {
		logrus.Errorf("Ds.MergePosts err: %s", err)
		return nil, web.ErrGetPostsFailed
	}
	return (*web.AdminAuditPostsResp)(base.PageRespFrom(formated, req.Page, req.PageSize, total)), nil
}

// AuditPostAction 审核·通过/拒绝/删除
func (s *auditSrv) AuditPostAction(req *web.AdminAuditPostReq) error {
	if req.User == nil {
		return web.ErrNoPermission
	}
	if req.Action == "reject" && len(req.Reason) == 0 {
		return xerror.InvalidParams.WithDetails("拒绝操作需要填写原因")
	}
	post, err := s.Ds.GetPostByID(req.PostID)
	if err != nil || post.Model == nil || post.ID <= 0 {
		return web.ErrGetPostFailed
	}
	oldStatus := uint8(post.AuditStatus)

	switch req.Action {
	case "approve":
		if post.AuditStatus != ms.PostAuditApproved {
			post.AuditStatus = ms.PostAuditApproved
			if err := s.Ds.UpdatePost(post); err != nil {
				logrus.Errorf("Ds.UpdatePost err: %s", err)
				return web.ErrAuditPostFailed
			}
			// 过审后进入搜索索引
			s.PushPostToSearch(post)
		}
	case "reject":
		post.AuditStatus = ms.PostAuditRejected
		if err := s.Ds.UpdatePost(post); err != nil {
			logrus.Errorf("Ds.UpdatePost err: %s", err)
			return web.ErrAuditPostFailed
		}
		// 移出搜索索引
		if err := s.DeleteSearchPost(post); err != nil {
			logrus.Errorf("s.DeleteSearchPost err: %s", err)
		}
	case "delete":
		mediaContents, err := s.Ds.DeletePost(post)
		if err != nil {
			logrus.Errorf("Ds.DeletePost err: %s", err)
			return web.ErrAuditPostFailed
		}
		deleteOssObjects(s.oss, mediaContents)
		if err := s.DeleteSearchPost(post); err != nil {
			logrus.Errorf("s.DeleteSearchPost err: %s", err)
		}
		// 软删后作者动态条栏缓存过期
		onTrendsActionEvent(_trendsActionDeleteTweet, post.UserID)
		onTweetActionEvent(_tweetActionDelete, post.UserID, "")
	}

	// 写审核日志(API响应依赖 同步写入)
	if err := s.Ds.CreateAuditLog(&ms.AuditLog{
		PostID:     post.ID,
		OperatorID: req.User.ID,
		Action:     req.Action,
		OldStatus:  oldStatus,
		NewStatus:  uint8(post.AuditStatus),
		Reason:     req.Reason,
	}); err != nil {
		// 日志失败不影响审核结果
		logrus.Errorf("Ds.CreateAuditLog err: %s", err)
	}
	// 过期广场索引与作者个人动态缓存
	cache.OnExpireIndexTweetEvent(post.UserID)
	return nil
}

// ListAuditLogs 审核日志
func (s *auditSrv) ListAuditLogs(req *web.AdminAuditLogsReq) (*web.AdminAuditLogsResp, error) {
	limit, offset := req.PageSize, (req.Page-1)*req.PageSize
	logs, total, err := s.Ds.ListAuditLogs(offset, limit)
	if err != nil {
		logrus.Errorf("Ds.ListAuditLogs err: %s", err)
		return nil, web.ErrGetPostsFailed
	}
	usernames := usernamesOf(s.Ds, logUserIDs(logs))
	items := make([]*web.AdminAuditLogItem, 0, len(logs))
	for _, l := range logs {
		items = append(items, &web.AdminAuditLogItem{
			ID:           l.ID,
			PostID:       l.PostID,
			OperatorID:   l.OperatorID,
			OperatorName: usernames[l.OperatorID],
			Action:       l.Action,
			OldStatus:    l.OldStatus,
			NewStatus:    l.NewStatus,
			Reason:       l.Reason,
			CreatedOn:    l.CreatedOn,
		})
	}
	return (*web.AdminAuditLogsResp)(base.PageRespFrom(items, req.Page, req.PageSize, total)), nil
}

// usernamesOf 批量获取用户名(宽松处理失败 缺失的用户名显示空)
func usernamesOf(ds core.DataService, ids []int64) map[int64]string {
	res := make(map[int64]string, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, exist := res[id]; exist {
			continue
		}
		if user, err := ds.GetUserByID(id); err == nil && user != nil {
			res[id] = user.Username
		}
	}
	return res
}

func logUserIDs(logs []*ms.AuditLog) []int64 {
	ids := make([]int64, 0, len(logs))
	for _, l := range logs {
		ids = append(ids, l.OperatorID)
	}
	return ids
}

func newAuditSrv(s *base.DaoServant, oss core.ObjectStorageService) api.Audit {
	return &auditSrv{
		DaoServant: s,
		oss:        oss,
	}
}
