// Copyright 2023 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package jinzhu

import (
	"fmt"
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

// SoftDeleteUser 用户管理: 软删除用户(标记is_del=1, 数据保留可恢复)
func (s *auditSrv) SoftDeleteUser(user *ms.User) error {
	return user.Delete(s.db)
}

func (s *auditSrv) ListAuditPosts(actor *ms.User, status int, offset, limit int) (res []*ms.Post, total int64, err error) {
	actor, err = freshReviewActor(s.db, actor)
	if err != nil {
		return
	}
	db := s.db.Model(&dbr.Post{}).Where("is_del = ?", 0)
	db = db.Where("id IN (?)", reviewTaskScope(s.db.Model(&ms.ReviewTask{}), actor).Select("target_id").Where("kind = ?", "post"))
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

func (s *auditSrv) ListAuditComments(actor *ms.User, status int, offset, limit int) (res []*dbr.AuditCommentRow, total int64, err error) {
	actor, err = freshReviewActor(s.db, actor)
	if err != nil {
		return
	}
	var tasks []*ms.ReviewTask
	if err = reviewTaskScope(s.db, actor).Select("kind", "target_id").Where("kind IN ?", []string{ms.ReviewComment, ms.ReviewReply, ms.ReviewCourseQuestion, ms.ReviewCourseAnswer}).Find(&tasks).Error; err != nil {
		return
	}
	ids := map[string][]int64{}
	for _, task := range tasks {
		ids[task.Kind] = append(ids[task.Kind], task.TargetID)
	}

	condC, condR, condCC, condCR := " AND c.id IN ?", " AND r.id IN ?", " AND cc.id IN ?", " AND cr.id IN ?"
	var args []any
	for _, kind := range []string{ms.ReviewComment, ms.ReviewReply, ms.ReviewCourseQuestion, ms.ReviewCourseAnswer} {
		args = append(args, ids[kind])
		if status >= 0 && status <= int(dbr.PostAuditRejected) {
			args = append(args, status)
		}
	}
	if status >= 0 && status <= int(dbr.PostAuditRejected) {
		condC += " AND c.audit_status = ?"
		condR += " AND r.audit_status = ?"
		condCC += " AND cc.audit_status = ?"
		condCR += " AND cr.audit_status = ?"
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
		s.db.NamingStrategy.TableName("comment"), condC,
		s.db.NamingStrategy.TableName("comment_reply"), s.db.NamingStrategy.TableName("comment"), condR,
		s.db.NamingStrategy.TableName("course_comment"), condCC,
		s.db.NamingStrategy.TableName("course_comment_reply"), s.db.NamingStrategy.TableName("course_comment"), condCR)
	if err = s.db.Raw("SELECT COUNT(*) FROM ("+union+") t", args...).Scan(&total).Error; err != nil {
		return
	}
	queryArgs := append(append([]any{}, args...), limit, offset)
	err = s.db.Raw("SELECT * FROM ("+union+") t ORDER BY t.created_on DESC, t.id DESC LIMIT ? OFFSET ?", queryArgs...).Scan(&res).Error
	return
}

// ListAuditNicknames returns pending profiles in the actor's assignment scope.
func (s *auditSrv) ListAuditNicknames(actor *ms.User, offset, limit int) (res []*ms.User, total int64, err error) {
	actor, err = freshReviewActor(s.db, actor)
	if err != nil {
		return
	}
	db := s.db.Model(&dbr.User{}).Where("is_del = ? AND pending_nickname <> ''", 0)
	db = db.Where("id IN (?)", reviewTaskScope(s.db.Model(&ms.ReviewTask{}), actor).Select("target_id").Where("kind = ?", "nickname"))
	if err = db.Count(&total).Error; err != nil {
		return
	}
	if offset >= 0 && limit > 0 {
		db = db.Offset(offset).Limit(limit)
	}
	err = db.Order("id DESC").Find(&res).Error
	return
}

// ListAuditAvatars returns pending profiles in the actor's assignment scope.
func (s *auditSrv) ListAuditAvatars(actor *ms.User, offset, limit int) (res []*ms.User, total int64, err error) {
	actor, err = freshReviewActor(s.db, actor)
	if err != nil {
		return
	}
	db := s.db.Model(&dbr.User{}).Where("is_del = ? AND pending_avatar <> ''", 0)
	db = db.Where("id IN (?)", reviewTaskScope(s.db.Model(&ms.ReviewTask{}), actor).Select("target_id").Where("kind = ?", "avatar"))
	if err = db.Count(&total).Error; err != nil {
		return
	}
	if offset >= 0 && limit > 0 {
		db = db.Offset(offset).Limit(limit)
	}
	err = db.Order("id DESC").Find(&res).Error
	return
}

func (s *auditSrv) ListAuditLogs(actor *ms.User, offset, limit int) (res []*ms.AuditLog, total int64, err error) {
	actor, err = freshReviewActor(s.db, actor)
	if err != nil {
		return
	}
	db := s.db.Model(&dbr.AuditLog{}).Where("is_del = ?", 0)
	if !actor.HasPermission("audit.view_all") {
		db = db.Where("operator_id = ?", actor.ID)
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
