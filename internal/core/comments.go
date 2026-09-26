// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package core

import (
	"github.com/BZYA-Community/WebsiteCore/internal/core/cs"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
)

// CommentService 评论检索服务
type CommentService interface {
	// viewerId/viewerIsAuditor: 审核视角控制可见性——
	//   游客: 仅已过审(audit_status=1)
	//   登录用户: 已过审 + 本人发布的任意状态(作者可见自己的待审/未过审评论)
	//   审核员/管理员: 全部
	GetComments(tweetId int64, style cs.StyleCommentType, viewerId int64, viewerIsAuditor bool, limit int, offset int) ([]*ms.Comment, int64, error)
	GetCommentByID(id int64) (*ms.Comment, error)
	GetCommentReplyByID(id int64) (*ms.CommentReply, error)
	// GetCommentRepliesByReplyIDs 批量按回复id取回复(供列表场景一次取回避免逐行查询)
	// 软删除的不返回 查询失败返回错误 缺失的id不在返回中
	GetCommentRepliesByReplyIDs(ids []int64) ([]*ms.CommentReply, error)
	GetCommentContentsByIDs(ids []int64) ([]*ms.CommentContent, error)
	GetCommentRepliesByID(ids []int64, viewerId int64, viewerIsAuditor bool) ([]*ms.CommentReplyFormated, error)
	GetCommentThumbsMap(userId int64, tweetId int64) (cs.CommentThumbsMap, cs.CommentThumbsMap, error)
}

// CommentManageService 评论管理服务
type CommentManageService interface {
	HighlightComment(userId, commentId int64) (int8, error)
	DeleteComment(comment *ms.Comment) error
	CreateComment(comment *ms.Comment) (*ms.Comment, error)
	CreateCommentReply(reply *ms.CommentReply) (*ms.CommentReply, error)
	DeleteCommentReply(reply *ms.CommentReply) error
	CreateCommentContent(content *ms.CommentContent) (*ms.CommentContent, error)
	ThumbsUpComment(userId int64, tweetId, commentId int64) error
	ThumbsDownComment(userId int64, tweetId, commentId int64) error
	ThumbsUpReply(userId int64, tweetId, commentId, replyId int64) error
	ThumbsDownReply(userId int64, tweetId, commentId, replyId int64) error
}
