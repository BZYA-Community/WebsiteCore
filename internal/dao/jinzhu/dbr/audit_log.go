// Copyright 2023 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package dbr

import (
	"gorm.io/gorm"
)

// AuditLog 内容审核操作日志
type AuditLog struct {
	*Model
	PostID     int64  `json:"post_id"`
	OperatorID int64  `json:"operator_id"`
	Action     string `json:"action"` // approve通过 reject拒绝 delete删除
	OldStatus  uint8  `json:"old_status"`
	NewStatus  uint8  `json:"new_status"`
	Reason     string `json:"reason"`
}

func (l *AuditLog) Create(db *gorm.DB) (*AuditLog, error) {
	err := db.Create(&l).Error
	return l, err
}

// AuditCommentRow 评论审核队列行(评论与回复UNION合并后的投影)
type AuditCommentRow struct {
	ID          int64      `json:"id"`
	CommentType int8       `json:"comment_type"` // 0评论 1回复
	PostID      int64      `json:"post_id"`
	CommentID   int64      `json:"comment_id"` // 回复所属评论ID(评论自身为0)
	UserID      int64      `json:"user_id"`
	AuditStatus PostAuditT `json:"audit_status"`
	CreatedOn   int64      `json:"created_on"`
}

func (l *AuditLog) List(db *gorm.DB, conditions *ConditionsT, offset, limit int) ([]*AuditLog, error) {
	var logs []*AuditLog
	if offset >= 0 && limit > 0 {
		db = db.Offset(offset).Limit(limit)
	}
	for k, v := range *conditions {
		if k == "ORDER" {
			db = db.Order(v)
		} else {
			db = db.Where(k, v)
		}
	}
	if err := db.Where("is_del = ?", 0).Find(&logs).Error; err != nil {
		return nil, err
	}
	return logs, nil
}

func (l *AuditLog) Count(db *gorm.DB, conditions *ConditionsT) (int64, error) {
	var count int64
	for k, v := range *conditions {
		if k == "ORDER" {
			db = db.Order(v)
		} else {
			db = db.Where(k, v)
		}
	}
	if err := db.Model(l).Where("is_del = ?", 0).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}
