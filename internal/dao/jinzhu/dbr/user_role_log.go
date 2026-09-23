// Copyright 2023 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package dbr

import (
	"gorm.io/gorm"
)

// UserRoleLog 用户角色变更日志
type UserRoleLog struct {
	*Model
	UserID     int64  `json:"user_id"`
	OperatorID int64  `json:"operator_id"`
	OldRoles   string `json:"old_roles"`
	NewRoles   string `json:"new_roles"`
	Action     string `json:"action"`
}

func (l *UserRoleLog) Create(db *gorm.DB) (*UserRoleLog, error) {
	err := db.Create(&l).Error
	return l, err
}

func (l *UserRoleLog) List(db *gorm.DB, conditions *ConditionsT, offset, limit int) ([]*UserRoleLog, error) {
	var logs []*UserRoleLog
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
