// Copyright 2023 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package web

import (
	"fmt"
	"strings"

	api "github.com/BZYA-Community/WebsiteCore/auto/api/v1"
	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/cache"
	"github.com/BZYA-Community/WebsiteCore/internal/model/web"
	"github.com/BZYA-Community/WebsiteCore/internal/servants/base"
	"github.com/BZYA-Community/WebsiteCore/internal/servants/chain"
	"github.com/BZYA-Community/WebsiteCore/pkg/xerror"
	"github.com/gin-gonic/gin"
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

// AuditPostAction 审核·通过/拒绝 (审核无权直接删除帖子 删除由作者自行操作)
func (s *auditSrv) AuditPostAction(req *web.AdminAuditPostReq) error {
	if req.User == nil {
		return web.ErrNoPermission
	}
	if req.Action != "approve" && req.Action != "reject" {
		return xerror.InvalidParams.WithDetails("仅支持通过/拒绝操作")
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
		// 拒绝: 标记未通过并打回私密(仅作者可见) 作者可将可见性重新设为非私密再次提交审核
		post.AuditStatus = ms.PostAuditRejected
		post.Visibility = ms.PostVisitPrivate
		if err := s.Ds.UpdatePost(post); err != nil {
			logrus.Errorf("Ds.UpdatePost err: %s", err)
			return web.ErrAuditPostFailed
		}
		// 移出搜索索引
		if err := s.DeleteSearchPost(post); err != nil {
			logrus.Errorf("s.DeleteSearchPost err: %s", err)
		}
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
	// 审核结果站内信通知作者
	if req.Action == "approve" {
		s.notifyAuditResult(post, true, "")
	} else {
		s.notifyAuditResult(post, false, req.Reason)
	}
	return nil
}

// notifyAuditResult 审核结果以系统消息通知帖子作者
func (s *auditSrv) notifyAuditResult(post *ms.Post, approved bool, reason string) {
	summary := auditPostSummary(s.Ds, post)
	content := ""
	if approved {
		content = fmt.Sprintf("您发布的动态[%s]已通过审核，现已对他人可见。", summary)
	} else {
		// p_message.content 列长255 限制原因长度以保留重新提交指引
		if r := []rune(reason); len(r) > 120 {
			reason = string(r[:120]) + "…"
		}
		content = fmt.Sprintf("您发布的动态[%s]审核未通过。原因：%s。您可将该动态的可见性重新设为公开或非私密，即可再次提交审核。", summary, reason)
	}
	brief := "动态审核通过"
	if !approved {
		brief = "动态审核未通过"
	}
	onCreateMessageEvent(&ms.Message{
		ReceiverUserID: post.UserID,
		Type:           ms.MsgTypeSystem,
		Brief:          brief,
		Content:        content,
		PostID:         post.ID,
	})
}

// auditPostSummary 提取帖子首段文字内容作摘要(无文字时退回帖子ID)
func auditPostSummary(ds core.DataService, post *ms.Post) string {
	contents, err := ds.GetPostContentsByIDs([]int64{post.ID})
	if err == nil {
		for _, c := range contents {
			if c.Type == ms.ContentTypeTitle || c.Type == ms.ContentTypeText || c.Type == ms.ContentTypeMarkdown {
				s := strings.TrimSpace(c.Content)
				if c.Type == ms.ContentTypeMarkdown {
					s = markdownSummaryText(s)
				}
				if s != "" {
					if len([]rune(s)) > 30 {
						return string([]rune(s)[:30]) + "…"
					}
					return s
				}
			}
		}
	}
	return fmt.Sprintf("#%d", post.ID)
}

// markdownSummaryText 去除Markdown语法标记提取摘要文本(通知等纯文本场景)
func markdownSummaryText(s string) string {
	s = strings.ReplaceAll(s, "```", "")
	lines := strings.Split(s, "\n")
	kept := make([]string, 0, len(lines))
	for _, ln := range lines {
		ln = strings.TrimLeft(ln, "#> ")
		ln = strings.TrimLeft(ln, "-*+ ")
		if strings.TrimSpace(ln) == "" {
			continue
		}
		kept = append(kept, ln)
	}
	s = strings.Join(kept, " ")
	replacer := strings.NewReplacer("**", "", "*", "", "__", "", "_", "", "`", "")
	return strings.TrimSpace(replacer.Replace(s))
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
