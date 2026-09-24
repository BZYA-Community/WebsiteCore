// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package web

import (
	"github.com/BZYA-Community/WebsiteCore/internal/model/joint"
)

// GetChatContactsReq 私信会话列表
type GetChatContactsReq struct {
	SimpleInfo `json:"-" binding:"-"`
}

// ChatContactItem 会话条目(user_id=0 为系统联系人)
type ChatContactItem struct {
	UserID      int64    `json:"user_id"`
	Username    string   `json:"username"`
	Nickname    string   `json:"nickname"`
	Avatar      string   `json:"avatar"`
	Roles       []string `json:"roles"`
	Identity    string   `json:"identity"`
	LastContent string   `json:"last_content"`
	LastTime    int64    `json:"last_time"`
	LastFromMe  bool     `json:"last_from_me"`
	Unread      int64    `json:"unread"`
}

type GetChatContactsResp struct {
	System   ChatContactItem   `json:"system"`
	Contacts []ChatContactItem `json:"contacts"`
}

// GetChatHistoryReq 聊天历史(user_id=0 为系统会话)
type GetChatHistoryReq struct {
	BaseInfo   `json:"-" binding:"-"`
	SimpleInfo `json:"-" binding:"-"`
	joint.BasePageInfo
	UserID int64 `form:"user_id"`
}

// ChatHistoryItem 聊天消息(系统会话带 type/post_id/comment_id 供前端跳转)
type ChatHistoryItem struct {
	ID             int64  `json:"id"`
	SenderID       int64  `json:"sender_id"`
	SenderName     string `json:"sender_name,omitempty"`
	SenderUsername string `json:"sender_username,omitempty"`
	Brief          string `json:"brief,omitempty"`
	Content        string `json:"content"`
	Type           int8   `json:"type"`
	PostID         int64  `json:"post_id,omitempty"`
	CommentID      int64  `json:"comment_id,omitempty"`
	Timestamp      int64  `json:"timestamp"`
	Seen           bool   `json:"seen"`
}

type GetChatHistoryResp struct {
	Messages   []ChatHistoryItem `json:"messages"`
	Peer       *ChatContactItem  `json:"peer,omitempty"`
	CanSend    bool              `json:"can_send"`
	CanSendTip string            `json:"can_send_tip"`
	TotalRows  int64             `json:"total_rows"`
}

// SendChatMessageReq 发送私信(user_id=0 系统会话会返回只读错误, 故不加 required)
type SendChatMessageReq struct {
	BaseInfo `json:"-" binding:"-"`
	UserID   int64  `json:"user_id"`
	Content  string `json:"content" binding:"required"`
}

type SendChatMessageResp struct {
	MessageID int64 `json:"message_id"`
}
