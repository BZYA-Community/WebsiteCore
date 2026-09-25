// Copyright 2023 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package web

import (
	"fmt"
	"regexp"
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

// commentMentionRegex 从评论文本中提取@用户名
var commentMentionRegex = regexp.MustCompile(`@([a-zA-Z0-9][a-zA-Z0-9_\-]{0,29})`)

// ListAuditComments 评论审核队列(评论与回复合并)
func (s *auditSrv) ListAuditComments(req *web.AdminAuditCommentsReq) (*web.AdminAuditCommentsResp, error) {
	limit, offset := req.PageSize, (req.Page-1)*req.PageSize
	rows, total, err := s.Ds.ListAuditComments(req.Status, offset, limit)
	if err != nil {
		logrus.Errorf("Ds.ListAuditComments err: %s", err)
		return nil, web.ErrGetPostsFailed
	}
	userIds := make([]int64, 0, len(rows))
	commentIds := make([]int64, 0, len(rows))
	replyIds := make([]int64, 0, len(rows))
	for _, row := range rows {
		userIds = append(userIds, row.UserID)
		if row.CommentType == 0 {
			commentIds = append(commentIds, row.ID)
		} else {
			replyIds = append(replyIds, row.ID)
		}
	}
	users, _ := s.Ds.GetUsersByIDs(userIds)
	userMap := make(map[int64]*ms.User, len(users))
	for _, u := range users {
		userMap[u.ID] = u
	}
	contentMap := make(map[int64]string, len(commentIds))
	if len(commentIds) > 0 {
		if contents, err := s.Ds.GetCommentContentsByIDs(commentIds); err == nil {
			for _, c := range contents {
				switch c.Type {
				case ms.ContentTypeImage:
					contentMap[c.CommentID] += "[图片]"
				case ms.ContentTypeVideo:
					contentMap[c.CommentID] += "[视频]"
				case ms.ContentTypeAttachment, ms.ContentTypeChargeAttachment:
					contentMap[c.CommentID] += "[附件]"
				default:
					contentMap[c.CommentID] += c.Content
				}
			}
		}
	}
	items := make([]*web.AdminAuditCommentItem, 0, len(rows))
	for _, row := range rows {
		item := &web.AdminAuditCommentItem{
			ID:          row.ID,
			CommentType: int(row.CommentType),
			PostID:      row.PostID,
			CommentID:   row.CommentID,
			Content:     auditBriefText(contentMap[row.ID]),
			AuditStatus: int(row.AuditStatus),
			CreatedOn:   row.CreatedOn,
		}
		if row.CommentType == 1 {
			if reply, err := s.Ds.GetCommentReplyByID(row.ID); err == nil {
				item.Content = auditBriefText(reply.Content)
			}
		}
		if u, exist := userMap[row.UserID]; exist {
			item.User = &web.AdminAuditUserBrief{ID: u.ID, Nickname: u.Nickname, Username: u.Username}
		}
		items = append(items, item)
	}
	return (*web.AdminAuditCommentsResp)(base.PageRespFrom(items, req.Page, req.PageSize, total)), nil
}

// auditBriefText 审核条目内容摘要(超长截断)
func auditBriefText(s string) string {
	if r := []rune(strings.TrimSpace(s)); len(r) > 60 {
		return string(r[:60]) + "…"
	} else if len(r) == 0 {
		return "(无文字内容)"
	} else {
		return string(r)
	}
}

// AuditCommentAction 评论审核·通过/拒绝(评论与回复)
func (s *auditSrv) AuditCommentAction(req *web.AdminAuditCommentReq) error {
	if req.User == nil {
		return web.ErrNoPermission
	}
	if req.Action == "reject" && len(req.Reason) == 0 {
		return xerror.InvalidParams.WithDetails("拒绝操作需要填写原因")
	}
	newStatus := int(ms.PostAuditApproved)
	if req.Action == "reject" {
		newStatus = int(ms.PostAuditRejected)
	}
	var (
		oldStatus int
		err       error
	)
	if req.CommentType == 0 {
		oldStatus, err = s.Ds.UpdateCommentAuditStatus(req.ID, newStatus)
	} else {
		oldStatus, err = s.Ds.UpdateCommentReplyAuditStatus(req.ID, newStatus)
	}
	if err != nil {
		logrus.Errorf("Ds.UpdateCommentAuditStatus err: %s", err)
		return web.ErrAuditCommentFailed
	}
	if oldStatus == newStatus {
		// 状态未变化 幂等返回
		return nil
	}

	action := "comment_" + req.Action
	if req.CommentType == 1 {
		action = "reply_" + req.Action
	}
	var post *ms.Post
	if req.CommentType == 0 {
		if post = s.applyCommentAuditEffects(req.ID, oldStatus, newStatus, req.Reason); post == nil {
			return nil
		}
	} else {
		if post = s.applyReplyAuditEffects(req.ID, oldStatus, newStatus, req.Reason); post == nil {
			return nil
		}
	}

	// 写审核日志(失败不影响审核结果)
	if err := s.Ds.CreateAuditLog(&ms.AuditLog{
		PostID:     post.ID,
		OperatorID: req.User.ID,
		Action:     action,
		OldStatus:  uint8(oldStatus),
		NewStatus:  uint8(newStatus),
		Reason:     req.Reason,
	}); err != nil {
		logrus.Errorf("Ds.CreateAuditLog err: %s", err)
	}
	return nil
}

// applyCommentAuditEffects 评论审核状态变更的联动: 帖子计数/索引/延迟通知/缓存
func (s *auditSrv) applyCommentAuditEffects(commentId int64, oldStatus, newStatus int, reason string) *ms.Post {
	comment, err := s.Ds.GetCommentByID(commentId)
	if err != nil || comment.Model == nil || comment.ID <= 0 {
		logrus.Errorf("auditSrv GetCommentByID[%d] err: %v", commentId, err)
		return nil
	}
	post, err := s.Ds.GetPostByID(comment.PostID)
	if err != nil {
		logrus.Errorf("auditSrv GetPostByID[%d] err: %s", comment.PostID, err)
		return nil
	}
	switch {
	case newStatus == int(ms.PostAuditApproved):
		// 过审: 补记评论数/索引 并补发创建时被延迟的通知
		post.CommentCount++
		post.LatestRepliedOn = comment.CreatedOn
		if err := s.Ds.UpdatePost(post); err != nil {
			logrus.Errorf("Ds.UpdatePost err: %s", err)
		}
		s.PushPostToSearch(post)
		s.notifyCommentApproved(post, comment)
		s.notifyCommentAuditResult(comment.UserID, false, post.ID, commentBrief(s.Ds, comment), "", true)
	case oldStatus == int(ms.PostAuditApproved):
		// 由过审转为拒绝: 回减评论数
		post.CommentCount--
		if err := s.Ds.UpdatePost(post); err != nil {
			logrus.Errorf("Ds.UpdatePost err: %s", err)
		}
		s.notifyCommentAuditResult(comment.UserID, false, post.ID, commentBrief(s.Ds, comment), reason, false)
	default:
		// 待审->拒绝: 从未计数/通知过 仅通知结果
		s.notifyCommentAuditResult(comment.UserID, false, post.ID, commentBrief(s.Ds, comment), reason, false)
	}
	// 缓存处理
	onCommentActionEvent(comment.PostID, comment.ID, _commentActionAudit)
	return post
}

// applyReplyAuditEffects 回复审核状态变更的联动(父评论reply_count已在DAO内调整)
func (s *auditSrv) applyReplyAuditEffects(replyId int64, oldStatus, newStatus int, reason string) *ms.Post {
	reply, err := s.Ds.GetCommentReplyByID(replyId)
	if err != nil || reply.Model == nil || reply.ID <= 0 {
		logrus.Errorf("auditSrv GetCommentReplyByID[%d] err: %v", replyId, err)
		return nil
	}
	comment, err := s.Ds.GetCommentByID(reply.CommentID)
	if err != nil {
		logrus.Errorf("auditSrv GetCommentByID[%d] err: %s", reply.CommentID, err)
		return nil
	}
	post, err := s.Ds.GetPostByID(comment.PostID)
	if err != nil {
		logrus.Errorf("auditSrv GetPostByID[%d] err: %s", comment.PostID, err)
		return nil
	}
	switch {
	case newStatus == int(ms.PostAuditApproved):
		post.CommentCount++
		post.LatestRepliedOn = reply.CreatedOn
		if err := s.Ds.UpdatePost(post); err != nil {
			logrus.Errorf("Ds.UpdatePost err: %s", err)
		}
		s.PushPostToSearch(post)
		s.notifyReplyApproved(post, comment, reply)
		s.notifyCommentAuditResult(reply.UserID, true, post.ID, auditBriefText(reply.Content), "", true)
	case oldStatus == int(ms.PostAuditApproved):
		post.CommentCount--
		if err := s.Ds.UpdatePost(post); err != nil {
			logrus.Errorf("Ds.UpdatePost err: %s", err)
		}
		s.notifyCommentAuditResult(reply.UserID, true, post.ID, auditBriefText(reply.Content), reason, false)
	default:
		s.notifyCommentAuditResult(reply.UserID, true, post.ID, auditBriefText(reply.Content), reason, false)
	}
	// 缓存处理
	onCommentActionEvent(comment.PostID, comment.ID, _commentActionAudit)
	return post
}

// notifyCommentApproved 评论过审后补发创建时被延迟的通知(帖子作者+文中@的用户)
func (s *auditSrv) notifyCommentApproved(post *ms.Post, comment *ms.Comment) {
	postMaster, err := s.Ds.GetUserByID(post.UserID)
	if err == nil && postMaster.ID != comment.UserID {
		onCreateMessageEvent(&ms.Message{
			SenderUserID:   comment.UserID,
			ReceiverUserID: postMaster.ID,
			Type:           ms.MsgtypeComment,
			Brief:          "在泡泡中评论了你",
			PostID:         post.ID,
			CommentID:      comment.ID,
		})
	}
	contents, err := s.Ds.GetCommentContentsByIDs([]int64{comment.ID})
	if err != nil {
		return
	}
	notified := map[int64]bool{post.UserID: true, comment.UserID: true}
	for _, c := range contents {
		if c.Type != ms.ContentTypeText {
			continue
		}
		for _, m := range commentMentionRegex.FindAllStringSubmatch(c.Content, -1) {
			user, err := s.Ds.GetUserByUsername(m[1])
			if err != nil || notified[user.ID] {
				continue
			}
			notified[user.ID] = true
			onCreateMessageEvent(&ms.Message{
				SenderUserID:   comment.UserID,
				ReceiverUserID: user.ID,
				Type:           ms.MsgtypeComment,
				Brief:          "在泡泡评论中@了你",
				PostID:         post.ID,
				CommentID:      comment.ID,
			})
		}
	}
}

// notifyReplyApproved 回复过审后补发创建时被延迟的通知
func (s *auditSrv) notifyReplyApproved(post *ms.Post, comment *ms.Comment, reply *ms.CommentReply) {
	commentMaster, err1 := s.Ds.GetUserByID(comment.UserID)
	postMaster, err2 := s.Ds.GetUserByID(post.UserID)
	if err1 == nil && commentMaster.ID != reply.UserID {
		onCreateMessageEvent(&ms.Message{
			SenderUserID:   reply.UserID,
			ReceiverUserID: commentMaster.ID,
			Type:           ms.MsgTypeReply,
			Brief:          "在泡泡评论下回复了你",
			PostID:         post.ID,
			CommentID:      comment.ID,
			ReplyID:        reply.ID,
		})
	}
	if err2 == nil && postMaster.ID != reply.UserID && (err1 != nil || commentMaster.ID != postMaster.ID) {
		onCreateMessageEvent(&ms.Message{
			SenderUserID:   reply.UserID,
			ReceiverUserID: postMaster.ID,
			Type:           ms.MsgTypeReply,
			Brief:          "在泡泡评论下发布了新回复",
			PostID:         post.ID,
			CommentID:      comment.ID,
			ReplyID:        reply.ID,
		})
	}
	if reply.AtUserID > 0 {
		if user, err := s.Ds.GetUserByID(reply.AtUserID); err == nil && user.ID != reply.UserID &&
			(err1 != nil || commentMaster.ID != user.ID) && (err2 != nil || postMaster.ID != user.ID) {
			onCreateMessageEvent(&ms.Message{
				SenderUserID:   reply.UserID,
				ReceiverUserID: user.ID,
				Type:           ms.MsgTypeReply,
				Brief:          "在泡泡评论的回复中@了你",
				PostID:         post.ID,
				CommentID:      comment.ID,
				ReplyID:        reply.ID,
			})
		}
	}
}

// notifyCommentAuditResult 评论/回复审核结果以系统消息通知作者
func (s *auditSrv) notifyCommentAuditResult(receiverUserID int64, isReply bool, postID int64, summary, reason string, approved bool) {
	kind := "评论"
	if isReply {
		kind = "回复"
	}
	content, brief := "", ""
	if approved {
		brief = kind + "审核通过"
		content = fmt.Sprintf("你的%s[%s]已通过审核，现已对他人可见。", kind, summary)
	} else {
		if r := []rune(reason); len(r) > 120 {
			reason = string(r[:120]) + "…"
		}
		brief = kind + "审核未通过"
		content = fmt.Sprintf("你的%s[%s]审核未通过。原因：%s。", kind, summary, reason)
	}
	onCreateMessageEvent(&ms.Message{
		ReceiverUserID: receiverUserID,
		Type:           ms.MsgTypeSystem,
		Brief:          brief,
		Content:        content,
		PostID:         postID,
	})
}

// commentBrief 评论内容摘要
func commentBrief(ds core.DataService, comment *ms.Comment) string {
	contents, err := ds.GetCommentContentsByIDs([]int64{comment.ID})
	if err == nil {
		for _, c := range contents {
			switch c.Type {
			case ms.ContentTypeImage:
				continue
			case ms.ContentTypeVideo:
				continue
			default:
				if s := strings.TrimSpace(c.Content); s != "" {
					return auditBriefText(s)
				}
			}
		}
		if len(contents) > 0 {
			return "(媒体内容)"
		}
	}
	return fmt.Sprintf("#%d", comment.ID)
}

// ListAuditNicknames 昵称审核队列
func (s *auditSrv) ListAuditNicknames(req *web.AdminAuditNicknamesReq) (*web.AdminAuditNicknamesResp, error) {
	limit, offset := req.PageSize, (req.Page-1)*req.PageSize
	users, total, err := s.Ds.ListAuditNicknames(offset, limit)
	if err != nil {
		logrus.Errorf("Ds.ListAuditNicknames err: %s", err)
		return nil, web.ErrGetPostsFailed
	}
	items := make([]*web.AdminAuditNicknameItem, 0, len(users))
	for _, u := range users {
		items = append(items, &web.AdminAuditNicknameItem{
			UserID:          u.ID,
			Username:        u.Username,
			Nickname:        u.Nickname,
			PendingNickname: u.PendingNickname,
			CreatedOn:       u.CreatedOn,
		})
	}
	return (*web.AdminAuditNicknamesResp)(base.PageRespFrom(items, req.Page, req.PageSize, total)), nil
}

// AuditNicknameAction 昵称审核·通过/拒绝
func (s *auditSrv) AuditNicknameAction(req *web.AdminAuditNicknameReq) error {
	if req.User == nil {
		return web.ErrNoPermission
	}
	if req.Action == "reject" && len(req.Reason) == 0 {
		return xerror.InvalidParams.WithDetails("拒绝操作需要填写原因")
	}
	user, err := s.Ds.GetUserByID(req.UserID)
	if err != nil || user.Model == nil || user.ID <= 0 {
		return xerror.InvalidParams.WithDetails("用户不存在")
	}
	if user.PendingNickname == "" {
		return xerror.InvalidParams.WithDetails("该用户没有待审核的昵称变更")
	}
	pending := user.PendingNickname
	if req.Action == "approve" {
		if err := s.Ds.UpdateUserNickname(user, pending, ""); err != nil {
			logrus.Errorf("Ds.UpdateUserNickname err: %s", err)
			return web.ErrAuditNicknameFailed
		}
		// 缓存处理
		onChangeUsernameEvent(user.ID, user.Username)
		onCreateMessageEvent(&ms.Message{
			ReceiverUserID: user.ID,
			Type:           ms.MsgTypeSystem,
			Brief:          "昵称审核通过",
			Content:        fmt.Sprintf("你的昵称已变更为[%s]。", pending),
		})
	} else {
		if err := s.Ds.UpdateUserNickname(user, user.Nickname, ""); err != nil {
			logrus.Errorf("Ds.UpdateUserNickname err: %s", err)
			return web.ErrAuditNicknameFailed
		}
		if r := []rune(req.Reason); len(r) > 120 {
			req.Reason = string(r[:120]) + "…"
		}
		onCreateMessageEvent(&ms.Message{
			ReceiverUserID: user.ID,
			Type:           ms.MsgTypeSystem,
			Brief:          "昵称审核未通过",
			Content:        fmt.Sprintf("你的昵称变更[%s]审核未通过。原因：%s。", pending, req.Reason),
		})
	}
	// 写审核日志(失败不影响审核结果)
	if err := s.Ds.CreateAuditLog(&ms.AuditLog{
		OperatorID: req.User.ID,
		Action:     "nickname_" + req.Action,
		Reason:     req.Reason,
	}); err != nil {
		logrus.Errorf("Ds.CreateAuditLog err: %s", err)
	}
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
