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
	return gin.HandlersChain{chain.JWT(), chain.Authorize()}
}

// ListAuditPosts 审核队列
func (s *auditSrv) ListAuditPosts(req *web.AdminAuditPostsReq) (*web.AdminAuditPostsResp, error) {
	limit, offset := req.PageSize, (req.Page-1)*req.PageSize
	posts, total, err := s.Ds.ListAuditPosts(&ms.User{Model: &ms.Model{ID: req.Uid}}, req.Status, offset, limit)
	if err != nil {
		logrus.Errorf("Ds.ListAuditPosts err: %s", err)
		return nil, web.ErrGetPostsFailed
	}
	formated, err := s.Ds.MergePosts(posts)
	if err != nil {
		logrus.Errorf("Ds.MergePosts err: %s", err)
		return nil, web.ErrGetPostsFailed
	}
	for _, post := range formated {
		task, err := s.Ds.ReviewTaskForTarget(&ms.User{Model: &ms.Model{ID: req.Uid}}, ms.ReviewPost, post.ID)
		if err != nil {
			return nil, reviewError(err)
		}
		post.ReviewTask = task
	}
	return (*web.AdminAuditPostsResp)(base.PageRespFrom(formated, req.Page, req.PageSize, total)), nil
}

// AuditPostAction 审核·通过/拒绝 (审核无权直接删除帖子 删除由作者自行操作)
func (s *auditSrv) AuditPostAction(req *web.AdminAuditPostReq) error {
	result, err := s.decideTarget(req.User, ms.ReviewPost, req.PostID, req.TaskID, req.Revision, req.Action, req.Reason)
	if err != nil {
		return err
	}
	post, err := s.Ds.GetPostByID(result.Task.TargetID)
	if err != nil {
		return web.ErrGetPostFailed
	}
	if req.Action == "approve" {
		s.PushPostToSearch(post)
	} else if err := s.DeleteSearchPost(post); err != nil {
		logrus.WithError(err).Error("remove rejected post from search")
	}
	cache.OnExpireIndexTweetEvent(post.UserID)
	s.notifyAuditResult(post, req.Action == "approve", req.Reason)
	return nil
}

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
	rows, total, err := s.Ds.ListAuditComments(&ms.User{Model: &ms.Model{ID: req.Uid}}, req.Status, offset, limit)
	if err != nil {
		logrus.Errorf("Ds.ListAuditComments err: %s", err)
		return nil, web.ErrGetPostsFailed
	}
	userIds := make([]int64, 0, len(rows))
	commentIds := make([]int64, 0, len(rows))
	courseCommentIds := make([]int64, 0, len(rows))
	for _, row := range rows {
		userIds = append(userIds, row.UserID)
		if row.CommentType == 0 {
			commentIds = append(commentIds, row.ID)
		} else if row.CommentType == 2 {
			courseCommentIds = append(courseCommentIds, row.ID)
		}
	}
	users, _ := s.Ds.GetUsersByIDs(userIds)
	userMap := make(map[int64]*ms.User, len(users))
	for _, u := range users {
		userMap[u.ID] = u
	}
	contentMap := make(map[int64]string, len(commentIds))
	mediaMap := make(map[int64][]*web.PostContentItem, len(commentIds))
	if len(commentIds) > 0 {
		if contents, err := s.Ds.GetCommentContentsByIDs(commentIds); err == nil {
			for _, c := range contents {
				mediaMap[c.CommentID] = append(mediaMap[c.CommentID], &web.PostContentItem{Content: c.Content, Type: c.Type, Sort: c.Sort})
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
	// 课程评论内容(comment_type=2) 与帖子评论同一摘要逻辑
	courseContentMap := make(map[int64]string, len(courseCommentIds))
	courseMediaMap := make(map[int64][]*web.PostContentItem, len(courseCommentIds))
	if len(courseCommentIds) > 0 {
		if contents, err := s.Ds.GetCourseCommentContentsByIDs(courseCommentIds); err == nil {
			for _, c := range contents {
				courseMediaMap[c.CommentID] = append(courseMediaMap[c.CommentID], &web.PostContentItem{Content: c.Content, Type: c.Type, Sort: c.Sort})
				switch c.Type {
				case ms.ContentTypeImage:
					courseContentMap[c.CommentID] += "[图片]"
				case ms.ContentTypeVideo:
					courseContentMap[c.CommentID] += "[视频]"
				case ms.ContentTypeAttachment, ms.ContentTypeChargeAttachment:
					courseContentMap[c.CommentID] += "[附件]"
				default:
					courseContentMap[c.CommentID] += c.Content
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
			Content:     contentMap[row.ID],
			Contents:    mediaMap[row.ID],
			AuditStatus: int(row.AuditStatus),
			CreatedOn:   row.CreatedOn,
		}
		if row.CommentType == 1 {
			if reply, err := s.Ds.GetCommentReplyByID(row.ID); err == nil {
				item.Content = reply.Content
			}
		}
		if row.CommentType == 2 {
			item.Content = courseContentMap[row.ID]
			item.Contents = courseMediaMap[row.ID]
		}
		if row.CommentType == 3 {
			if reply, err := s.Ds.GetCourseCommentReplyByID(row.ID); err == nil {
				item.Content = reply.Content
			}
		}
		if u, exist := userMap[row.UserID]; exist {
			item.User = &web.AdminAuditUserBrief{ID: u.ID, Nickname: u.Nickname, Username: u.Username}
		}
		kinds := []string{ms.ReviewComment, ms.ReviewReply, ms.ReviewCourseQuestion, ms.ReviewCourseAnswer}
		item.ReviewTask, err = s.Ds.ReviewTaskForTarget(&ms.User{Model: &ms.Model{ID: req.Uid}}, kinds[row.CommentType], row.ID)
		if err != nil {
			return nil, reviewError(err)
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
	kinds := []string{ms.ReviewComment, ms.ReviewReply, ms.ReviewCourseQuestion, ms.ReviewCourseAnswer}
	if req.CommentType < 0 || req.CommentType >= len(kinds) {
		return xerror.InvalidParams
	}
	result, err := s.decideTarget(req.User, kinds[req.CommentType], req.ID, req.TaskID, req.Revision, req.Action, req.Reason)
	if err != nil {
		return err
	}
	approved := req.Action == "approve"
	if req.CommentType >= 2 {
		s.notifyCommentAuditResult(result.Task.AuthorID, req.CommentType == 3, 0, fmt.Sprintf("#%d", req.ID), req.Reason, approved)
		return nil
	}
	post, err := s.Ds.GetPostByID(result.ParentID)
	if err != nil {
		return web.ErrGetPostFailed
	}
	if req.CommentType == 0 {
		comment, err := s.Ds.GetCommentByID(req.ID)
		if err != nil {
			return web.ErrGetCommentFailed
		}
		if approved {
			s.notifyCommentApproved(post, comment)
		}
		s.notifyCommentAuditResult(comment.UserID, false, post.ID, commentBrief(s.Ds, comment), req.Reason, approved)
		onCommentActionEvent(post.ID, comment.ID, _commentActionAudit)
	} else {
		reply, err := s.Ds.GetCommentReplyByID(req.ID)
		if err != nil {
			return web.ErrGetCommentFailed
		}
		comment, err := s.Ds.GetCommentByID(reply.CommentID)
		if err != nil {
			return web.ErrGetCommentFailed
		}
		if approved {
			s.notifyReplyApproved(post, comment, reply)
		}
		s.notifyCommentAuditResult(reply.UserID, true, post.ID, auditBriefText(reply.Content), req.Reason, approved)
		onCommentActionEvent(post.ID, comment.ID, _commentActionAudit)
	}
	s.PushPostToSearch(post)
	return nil
}

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
	users, total, err := s.Ds.ListAuditNicknames(&ms.User{Model: &ms.Model{ID: req.Uid}}, offset, limit)
	if err != nil {
		logrus.Errorf("Ds.ListAuditNicknames err: %s", err)
		return nil, web.ErrGetPostsFailed
	}
	items := make([]*web.AdminAuditNicknameItem, 0, len(users))
	for _, u := range users {
		task, err := s.Ds.ReviewTaskForTarget(&ms.User{Model: &ms.Model{ID: req.Uid}}, ms.ReviewNickname, u.ID)
		if err != nil {
			return nil, reviewError(err)
		}
		items = append(items, &web.AdminAuditNicknameItem{ReviewTask: task,
			UserID:          u.ID,
			Username:        u.Username,
			Nickname:        u.Nickname,
			PendingNickname: task.Snapshot,
			CreatedOn:       u.CreatedOn,
		})
	}
	return (*web.AdminAuditNicknamesResp)(base.PageRespFrom(items, req.Page, req.PageSize, total)), nil
}

// AuditNicknameAction 昵称审核·通过/拒绝
func (s *auditSrv) AuditNicknameAction(req *web.AdminAuditNicknameReq) error {
	result, err := s.decideTarget(req.User, ms.ReviewNickname, req.UserID, req.TaskID, req.Revision, req.Action, req.Reason)
	if err != nil {
		return err
	}
	user, err := s.Ds.GetUserByID(req.UserID)
	if err != nil {
		return xerror.ServerError
	}
	onChangeUsernameEvent(user.ID, user.Username)
	brief, content := "昵称审核通过", fmt.Sprintf("你的昵称已变更为[%s]。", result.NewValue)
	if req.Action == "reject" {
		brief, content = "昵称审核未通过", "你的昵称变更未通过审核。原因："+auditBriefText(req.Reason)
	}
	onCreateMessageEvent(&ms.Message{ReceiverUserID: user.ID, Type: ms.MsgTypeSystem, Brief: brief, Content: content})
	return nil
}

func (s *auditSrv) ListAuditAvatars(req *web.AdminAuditAvatarsReq) (*web.AdminAuditAvatarsResp, error) {
	limit, offset := req.PageSize, (req.Page-1)*req.PageSize
	users, total, err := s.Ds.ListAuditAvatars(&ms.User{Model: &ms.Model{ID: req.Uid}}, offset, limit)
	if err != nil {
		logrus.Errorf("Ds.ListAuditAvatars err: %s", err)
		return nil, web.ErrGetPostsFailed
	}
	items := make([]*web.AdminAuditAvatarItem, 0, len(users))
	for _, u := range users {
		task, err := s.Ds.ReviewTaskForTarget(&ms.User{Model: &ms.Model{ID: req.Uid}}, ms.ReviewAvatar, u.ID)
		if err != nil {
			return nil, reviewError(err)
		}
		items = append(items, &web.AdminAuditAvatarItem{ReviewTask: task,
			UserID:        u.ID,
			Username:      u.Username,
			Avatar:        u.Avatar,
			PendingAvatar: task.Snapshot,
			CreatedOn:     u.CreatedOn,
		})
	}
	return (*web.AdminAuditAvatarsResp)(base.PageRespFrom(items, req.Page, req.PageSize, total)), nil
}

// AuditAvatarAction 头像审核·通过/拒绝
func (s *auditSrv) AuditAvatarAction(req *web.AdminAuditAvatarReq) error {
	_, err := s.decideTarget(req.User, ms.ReviewAvatar, req.UserID, req.TaskID, req.Revision, req.Action, req.Reason)
	if err != nil {
		return err
	}
	user, err := s.Ds.GetUserByID(req.UserID)
	if err != nil {
		return xerror.ServerError
	}
	onChangeUsernameEvent(user.ID, user.Username)
	brief, content := "头像审核通过", "你的新头像已通过审核并生效。"
	if req.Action == "reject" {
		brief, content = "头像审核未通过", "你的头像变更未通过审核。原因："+auditBriefText(req.Reason)
	}
	onCreateMessageEvent(&ms.Message{ReceiverUserID: user.ID, Type: ms.MsgTypeSystem, Brief: brief, Content: content})
	return nil
}

func (s *auditSrv) ListAuditLogs(req *web.AdminAuditLogsReq) (*web.AdminAuditLogsResp, error) {
	limit, offset := req.PageSize, (req.Page-1)*req.PageSize
	logs, total, err := s.Ds.ListAuditLogs(&ms.User{Model: &ms.Model{ID: req.Uid}}, offset, limit)
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
