// Copyright 2023 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package jinzhu

import (
	"strconv"
	"strings"

	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/jinzhu/dbr"
	"gorm.io/gorm"
)

type auditSrv struct {
	db *gorm.DB
}

func newAuditService(db *gorm.DB) core.SiteAdminService {
	return &auditSrv{db: db}
}

func (s *auditSrv) GetUsersByAdminQuery(keyword string, offset, limit int) (res []*ms.User, total int64, err error) {
	db := s.db.Model(&dbr.User{}).Where("is_del = ?", 0)
	kw := strings.TrimSpace(keyword)
	if kw != "" {
		like := "%" + kw + "%"
		if id, e := strconv.ParseInt(kw, 10, 64); e == nil {
			db = db.Where("id = ? OR username LIKE ? OR nickname LIKE ? OR phone LIKE ?", id, like, like, like)
		} else {
			db = db.Where("username LIKE ? OR nickname LIKE ? OR phone LIKE ?", like, like, like)
		}
	}
	if err = db.Count(&total).Error; err != nil {
		return
	}
	if offset >= 0 && limit > 0 {
		db = db.Offset(offset).Limit(limit)
	}
	err = db.Order("id ASC").Find(&res).Error
	return
}

func (s *auditSrv) ListAuditPosts(status int, offset, limit int) (res []*ms.Post, total int64, err error) {
	db := s.db.Model(&dbr.Post{}).Where("is_del = ?", 0)
	if status >= 0 && status <= int(dbr.PostAuditRejected) {
		db = db.Where("audit_status = ?", status)
	}
	if err = db.Count(&total).Error; err != nil {
		return
	}
	if offset >= 0 && limit > 0 {
		db = db.Offset(offset).Limit(limit)
	}
	err = db.Order("id DESC").Find(&res).Error
	return
}

func (s *auditSrv) CreateAuditLog(log *ms.AuditLog) error {
	_, err := log.Create(s.db)
	return err
}

func (s *auditSrv) ListAuditLogs(offset, limit int) (res []*ms.AuditLog, total int64, err error) {
	db := s.db.Model(&dbr.AuditLog{}).Where("is_del = ?", 0)
	if err = db.Count(&total).Error; err != nil {
		return
	}
	if offset >= 0 && limit > 0 {
		db = db.Offset(offset).Limit(limit)
	}
	err = db.Order("id DESC").Find(&res).Error
	return
}

func (s *auditSrv) CreateUserRoleLog(log *ms.UserRoleLog) error {
	_, err := log.Create(s.db)
	return err
}

func (s *auditSrv) ListUserRoleLogs(offset, limit int) (res []*ms.UserRoleLog, total int64, err error) {
	db := s.db.Model(&dbr.UserRoleLog{}).Where("is_del = ?", 0)
	if err = db.Count(&total).Error; err != nil {
		return
	}
	if offset >= 0 && limit > 0 {
		db = db.Offset(offset).Limit(limit)
	}
	err = db.Order("id DESC").Find(&res).Error
	return
}
