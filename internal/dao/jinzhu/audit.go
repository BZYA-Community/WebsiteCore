// Copyright 2023 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package jinzhu

import (
	"fmt"
	"strconv"
	"strings"
	"time"

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

// SoftDeleteUser 用户管理: 软删除用户(标记is_del=1, 数据保留可恢复)
func (s *auditSrv) SoftDeleteUser(user *ms.User) error {
	return user.Delete(s.db)
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

// ListAuditComments 评论审核队列: 评论与回复UNION合并按创建时间倒序分页
// status: 0待审核 1已通过 2未通过 -1全部
func (s *auditSrv) ListAuditComments(status int, offset, limit int) (res []*dbr.AuditCommentRow, total int64, err error) {
	condC, condR, condCC, condCR := "", "", "", ""
	var args []any
	if status >= 0 && status <= int(dbr.PostAuditRejected) {
		condC = " AND c.audit_status = ?"
		condR = " AND r.audit_status = ?"
		condCC = " AND cc.audit_status = ?"
		condCR = " AND cr.audit_status = ?"
		// 注意: 四个UNION分支的顺序参数, 追加顺序必须与SQL分支顺序一致
		args = append(args, status, status, status, status)
	}
	// comment_type: 0帖子评论 1帖子回复 2课程评论 3课程回复; post_id列在课程类型下承载course_id
	union := fmt.Sprintf(`SELECT c.id, 0 AS comment_type, c.post_id, 0 AS comment_id, c.user_id, c.audit_status, c.created_on
FROM %s c WHERE c.is_del = 0%s
UNION ALL
SELECT r.id, 1 AS comment_type, p.post_id, r.comment_id, r.user_id, r.audit_status, r.created_on
FROM %s r JOIN %s p ON r.comment_id = p.id WHERE r.is_del = 0%s
UNION ALL
SELECT cc.id, 2 AS comment_type, cc.course_id, 0 AS comment_id, cc.user_id, cc.audit_status, cc.created_on
FROM %s cc WHERE cc.is_del = 0%s
UNION ALL
SELECT cr.id, 3 AS comment_type, pc.course_id, cr.comment_id, cr.user_id, cr.audit_status, cr.created_on
FROM %s cr JOIN %s pc ON cr.comment_id = pc.id WHERE cr.is_del = 0%s`,
		_comment_, condC, _commentReply_, _comment_, condR, _courseComment_, condCC, _courseCommentReply_, _courseComment_, condCR)
	if err = s.db.Raw("SELECT COUNT(*) FROM ("+union+") t", args...).Scan(&total).Error; err != nil {
		return
	}
	queryArgs := append(append([]any{}, args...), limit, offset)
	err = s.db.Raw("SELECT * FROM ("+union+") t ORDER BY t.created_on DESC, t.id DESC LIMIT ? OFFSET ?", queryArgs...).Scan(&res).Error
	return
}

// UpdateCommentAuditStatus 更新评论审核状态 返回旧状态供上层联动帖子评论数/通知
func (s *auditSrv) UpdateCommentAuditStatus(id int64, status int) (oldStatus int, err error) {
	var comment dbr.Comment
	if err = s.db.Where("id = ? AND is_del = ?", id, 0).First(&comment).Error; err != nil {
		return
	}
	oldStatus = int(comment.AuditStatus)
	err = s.db.Model(&dbr.Comment{}).Where("id = ?", id).Updates(map[string]any{
		"audit_status": status,
		"modified_on":  time.Now().Unix(),
	}).Error
	return
}

// UpdateCommentReplyAuditStatus 更新回复审核状态 返回旧状态 并联动父评论reply_count
func (s *auditSrv) UpdateCommentReplyAuditStatus(id int64, status int) (oldStatus int, err error) {
	var reply dbr.CommentReply
	if err = s.db.Where("id = ? AND is_del = ?", id, 0).First(&reply).Error; err != nil {
		return
	}
	oldStatus = int(reply.AuditStatus)
	if err = s.db.Model(&dbr.CommentReply{}).Where("id = ?", id).Updates(map[string]any{
		"audit_status": status,
		"modified_on":  time.Now().Unix(),
	}).Error; err != nil {
		return
	}
	// 待审/未过审 -> 过审: 补记回复数; 过审 -> 拒绝: 回减
	switch {
	case oldStatus != int(dbr.PostAuditApproved) && status == int(dbr.PostAuditApproved):
		err = s.db.Table(_comment_).Where("id = ?", reply.CommentID).Update("reply_count", gorm.Expr("reply_count+1")).Error
	case oldStatus == int(dbr.PostAuditApproved) && status != int(dbr.PostAuditApproved):
		err = s.db.Table(_comment_).Where("id = ?", reply.CommentID).Update("reply_count", gorm.Expr("reply_count-1")).Error
	}
	return
}

// ListAuditNicknames 昵称审核队列: pending_nickname非空的用户
func (s *auditSrv) ListAuditNicknames(offset, limit int) (res []*ms.User, total int64, err error) {
	db := s.db.Model(&dbr.User{}).Where("is_del = ? AND pending_nickname <> ''", 0)
	if err = db.Count(&total).Error; err != nil {
		return
	}
	if offset >= 0 && limit > 0 {
		db = db.Offset(offset).Limit(limit)
	}
	err = db.Order("id DESC").Find(&res).Error
	return
}

// UpdateUserNickname 昵称审核结果落库(指定列更新 零值pending可清空)
func (s *auditSrv) UpdateUserNickname(user *ms.User, nickname, pendingNickname string) error {
	return s.db.Model(&dbr.User{}).Where("id = ?", user.ID).Updates(map[string]any{
		"nickname":         nickname,
		"pending_nickname": pendingNickname,
		"modified_on":      time.Now().Unix(),
	}).Error
}

// ListAuditAvatars 头像审核队列: pending_avatar非空的用户
func (s *auditSrv) ListAuditAvatars(offset, limit int) (res []*ms.User, total int64, err error) {
	db := s.db.Model(&dbr.User{}).Where("is_del = ? AND pending_avatar <> ''", 0)
	if err = db.Count(&total).Error; err != nil {
		return
	}
	if offset >= 0 && limit > 0 {
		db = db.Offset(offset).Limit(limit)
	}
	err = db.Order("id DESC").Find(&res).Error
	return
}

// UpdateUserAvatar 头像审核结果落库(指定列更新 零值pending可清空)
func (s *auditSrv) UpdateUserAvatar(user *ms.User, avatar, pendingAvatar string) error {
	return s.db.Model(&dbr.User{}).Where("id = ?", user.ID).Updates(map[string]any{
		"avatar":         avatar,
		"pending_avatar": pendingAvatar,
		"modified_on":    time.Now().Unix(),
	}).Error
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
