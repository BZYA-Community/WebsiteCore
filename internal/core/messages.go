// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package core

import (
	"github.com/BZYA-Community/WebsiteCore/internal/core/cs"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
)

// MessageService 消息服务
type MessageService interface {
	CreateMessage(msg *ms.Message) (*ms.Message, error)
	GetUnreadCount(userID int64) (int64, error)
	GetMessageByID(id int64) (*ms.Message, error)
	ReadMessage(message *ms.Message) error
	ReadAllMessage(userId int64) error
	GetMessages(userId int64, style cs.MessageStyle, limit, offset int) ([]*ms.MessageFormated, int64, error)

	// GetRecentWhispers 我参与的全部私信最近limit行(会话列表分组用)
	GetRecentWhispers(userID int64, limit int) ([]*ms.Message, error)
	// CountWhisperUnreadBySender 按发送方统计我的私信未读数(会话角标)
	CountWhisperUnreadBySender(userID int64) (map[int64]int64, error)
	// GetWhisperHistory 与某人的私信双向历史(id倒序分页)
	GetWhisperHistory(userID, otherID int64, limit, offset int) ([]*ms.Message, int64, error)
	// HasWhispered sender是否曾给receiver发过私信(首条限制判定)
	HasWhispered(senderID, receiverID int64) (bool, error)
	// ReadWhispersFrom 把某人发给我的私信全部标记已读(打开会话即读)
	ReadWhispersFrom(userID, senderID int64) error
	// ReadSystemMessages 系统会话全部标记已读(type 1动态/2评论/3回复/99系统)
	ReadSystemMessages(userID int64) error
	// CountSystemUnread 系统会话未读数(系统联系人角标)
	CountSystemUnread(userID int64) (int64, error)
}
