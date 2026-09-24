// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package jinzhu

import (
	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/core/cs"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/jinzhu/dbr"
	"gorm.io/gorm"
)

type messageSrv struct {
	db *gorm.DB
}

func newMessageService(db *gorm.DB) core.MessageService {
	return &messageSrv{
		db: db,
	}
}

func (s *messageSrv) CreateMessage(msg *ms.Message) (*ms.Message, error) {
	return msg.Create(s.db)
}

func (s *messageSrv) GetUnreadCount(userID int64) (int64, error) {
	return (&dbr.Message{}).CountUnread(s.db, userID)
}

func (s *messageSrv) GetMessageByID(id int64) (*ms.Message, error) {
	return (&dbr.Message{
		Model: &dbr.Model{
			ID: id,
		},
	}).Get(s.db)
}

func (s *messageSrv) ReadMessage(message *ms.Message) error {
	message.IsRead = 1
	return message.Update(s.db)
}

func (s *messageSrv) ReadAllMessage(userId int64) error {
	return s.db.Table(_message_).Where("receiver_user_id=? AND is_del=0", userId).Update("is_read", 1).Error
}

func (s *messageSrv) GetMessages(userId int64, style cs.MessageStyle, limit int, offset int) (res []*ms.MessageFormated, total int64, err error) {
	var messages []*dbr.Message
	db := s.db.Table(_message_)
	// 1动态，2评论，3回复，4私信，5好友申请，99系统通知'
	switch style {
	case cs.StyleMsgSystem:
		db = db.Where("receiver_user_id=? AND type IN (1, 2, 3, 99)", userId)
	case cs.StyleMsgWhisper:
		db = db.Where("(receiver_user_id=? OR sender_user_id=?) AND type=4", userId, userId)
	case cs.StyleMsgRequesting:
		db = db.Where("receiver_user_id=? AND type=5", userId)
	case cs.StyleMsgUnread:
		db = db.Where("receiver_user_id=? AND is_read=0", userId)
	case cs.StyleMsgAll:
		fallthrough
	default:
		db = db.Where("receiver_user_id=? OR (sender_user_id=? AND type=4)", userId, userId)
	}
	if err = db.Count(&total).Error; err != nil || total == 0 {
		return
	}
	if offset >= 0 && limit > 0 {
		db = db.Limit(limit).Offset(offset)
	}
	if err = db.Order("id DESC").Find(&messages).Error; err != nil {
		return
	}
	for _, message := range messages {
		res = append(res, message.Format())
	}
	return
}

// GetRecentWhispers 我参与的全部私信最近limit行(会话列表分组用)
func (s *messageSrv) GetRecentWhispers(userID int64, limit int) (res []*ms.Message, err error) {
	err = s.db.Table(_message_).
		Where("(receiver_user_id=? OR sender_user_id=?) AND type=4 AND is_del=0", userID, userID).
		Order("id DESC").Limit(limit).Find(&res).Error
	return
}

// CountWhisperUnreadBySender 按发送方统计我的私信未读数(会话角标)
func (s *messageSrv) CountWhisperUnreadBySender(userID int64) (map[int64]int64, error) {
	type row struct {
		SenderUserID int64 `gorm:"column:sender_user_id"`
		Count        int64 `gorm:"column:count"`
	}
	var rows []row
	if err := s.db.Table(_message_).
		Select("sender_user_id, COUNT(*) AS count").
		Where("receiver_user_id=? AND type=4 AND is_read=0 AND is_del=0", userID).
		Group("sender_user_id").Find(&rows).Error; err != nil {
		return nil, err
	}
	res := make(map[int64]int64, len(rows))
	for _, r := range rows {
		res[r.SenderUserID] = r.Count
	}
	return res, nil
}

// GetWhisperHistory 与某人的私信双向历史(id倒序分页, servant层翻转为正序)
func (s *messageSrv) GetWhisperHistory(userID, otherID int64, limit, offset int) (res []*ms.Message, total int64, err error) {
	db := s.db.Table(_message_).
		Where("((sender_user_id=? AND receiver_user_id=?) OR (sender_user_id=? AND receiver_user_id=?)) AND type=4 AND is_del=0",
			userID, otherID, otherID, userID)
	if err = db.Count(&total).Error; err != nil || total == 0 {
		return
	}
	if offset >= 0 && limit > 0 {
		db = db.Limit(limit).Offset(offset)
	}
	err = db.Order("id DESC").Find(&res).Error
	return
}

// HasWhispered sender是否曾给receiver发过私信(首条限制判定)
func (s *messageSrv) HasWhispered(senderID, receiverID int64) (bool, error) {
	var count int64
	err := s.db.Table(_message_).
		Where("sender_user_id=? AND receiver_user_id=? AND type=4 AND is_del=0", senderID, receiverID).
		Limit(1).Count(&count).Error
	return count > 0, err
}

// ReadWhispersFrom 把某人发给我的私信全部标记已读(打开会话即读)
func (s *messageSrv) ReadWhispersFrom(userID, senderID int64) error {
	return s.db.Table(_message_).
		Where("receiver_user_id=? AND sender_user_id=? AND type=4 AND is_read=0 AND is_del=0", userID, senderID).
		Update("is_read", 1).Error
}

// ReadSystemMessages 系统会话全部标记已读(type 1动态/2评论/3回复/99系统)
func (s *messageSrv) ReadSystemMessages(userID int64) error {
	return s.db.Table(_message_).
		Where("receiver_user_id=? AND type IN (1, 2, 3, 99) AND is_read=0 AND is_del=0", userID).
		Update("is_read", 1).Error
}

// CountSystemUnread 系统会话未读数(系统联系人角标)
func (s *messageSrv) CountSystemUnread(userID int64) (count int64, err error) {
	err = s.db.Table(_message_).
		Where("receiver_user_id=? AND type IN (1, 2, 3, 99) AND is_read=0 AND is_del=0", userID).
		Count(&count).Error
	return
}
