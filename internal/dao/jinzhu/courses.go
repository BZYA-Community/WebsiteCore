// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package jinzhu

import (
	"time"

	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/jinzhu/dbr"
	"gorm.io/gorm"
)

var (
	_ core.CourseService       = (*courseSrv)(nil)
	_ core.CourseManageService = (*courseManageSrv)(nil)
)

type courseSrv struct {
	db *gorm.DB
}

type courseManageSrv struct {
	db *gorm.DB
}

func newCourseService(db *gorm.DB) core.CourseService {
	return &courseSrv{db: db}
}

func newCourseManageService(db *gorm.DB) core.CourseManageService {
	return &courseManageSrv{db: db}
}

func (s *courseSrv) GetCourseByID(id int64) (*ms.Course, error) {
	course := &dbr.Course{
		Model: &dbr.Model{
			ID: id,
		},
	}
	return course.Get(s.db)
}

func (s *courseSrv) GetCourseGroupByID(id int64) (*ms.CourseGroup, error) {
	group := &dbr.CourseGroup{
		Model: &dbr.Model{
			ID: id,
		},
	}
	return group.Get(s.db)
}

func (s *courseSrv) ListCourseGroups() ([]*ms.CourseGroupFormated, error) {
	groups, err := (&dbr.CourseGroup{}).List(s.db)
	if err != nil {
		return nil, err
	}
	// 一次分组统计避免N+1
	type groupCount struct {
		GroupID int64
		Cnt     int64
	}
	var counts []groupCount
	if err = s.db.Model(&dbr.Course{}).Select("group_id, COUNT(*) AS cnt").
		Where("is_del = ?", 0).Group("group_id").Find(&counts).Error; err != nil {
		return nil, err
	}
	countMap := make(map[int64]int64, len(counts))
	for _, c := range counts {
		countMap[c.GroupID] = c.Cnt
	}
	res := make([]*ms.CourseGroupFormated, 0, len(groups))
	for _, g := range groups {
		res = append(res, g.Format(countMap[g.ID]))
	}
	return res, nil
}

func (s *courseSrv) ListCourses(groupId int64, keyword string, offset, limit int) (res []*ms.Course, total int64, err error) {
	db := s.db.Model(&dbr.Course{}).Where("is_del = ?", 0)
	if groupId > 0 {
		db = db.Where("group_id = ?", groupId)
	}
	if keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where("title LIKE ? OR intro LIKE ?", like, like)
	}
	if err = db.Count(&total).Error; err != nil {
		return
	}
	err = db.Order("id DESC").Offset(offset).Limit(limit).Find(&res).Error
	return
}

// addCourseCommentAuditScope 课程评论审核可见范围与帖子评论同口径(见addCommentAuditScope)
func addCourseCommentAuditScope(db *gorm.DB, viewerId int64, viewerIsAuditor bool) *gorm.DB {
	return addCommentAuditScope(db, viewerId, viewerIsAuditor)
}

func (s *courseSrv) GetCourseComments(courseId, viewerId int64, viewerIsAuditor bool, limit, offset int) (res []*ms.CourseComment, total int64, err error) {
	db := s.db.Model(&dbr.CourseComment{}).Where("course_id = ?", courseId)
	db = addCourseCommentAuditScope(db, viewerId, viewerIsAuditor)
	if err = db.Count(&total).Error; err != nil {
		return
	}
	err = db.Order("id DESC").Limit(limit).Offset(offset).Find(&res).Error
	return
}

func (s *courseSrv) GetCourseCommentByID(id int64) (*ms.CourseComment, error) {
	comment := &dbr.CourseComment{
		Model: &dbr.Model{
			ID: id,
		},
	}
	return comment.Get(s.db)
}

func (s *courseSrv) GetCourseCommentReplyByID(id int64) (*ms.CourseCommentReply, error) {
	reply := &dbr.CourseCommentReply{
		Model: &dbr.Model{
			ID: id,
		},
	}
	return reply.Get(s.db)
}

func (s *courseSrv) GetCourseCommentContentsByIDs(ids []int64) (res []*ms.CourseCommentContent, err error) {
	err = s.db.Where("comment_id IN ? AND is_del = ?", ids, 0).Find(&res).Error
	return
}

func (s *courseSrv) GetCourseCommentRepliesByID(ids []int64, viewerId int64, viewerIsAuditor bool) ([]*ms.CourseCommentReplyFormated, error) {
	var replies []*dbr.CourseCommentReply
	err := s.db.Where("comment_id IN ? AND is_del = ?", ids, 0).Order("id ASC").Find(&replies).Error
	if err != nil {
		return nil, err
	}
	// 审核可见范围与评论同口径(回复量以页内评论为界 内存过滤即可)
	if !viewerIsAuditor {
		kept := make([]*dbr.CourseCommentReply, 0, len(replies))
		for _, reply := range replies {
			if reply.AuditStatus == dbr.PostAuditApproved || reply.UserID == viewerId {
				kept = append(kept, reply)
			}
		}
		replies = kept
	}
	userIds := []int64{}
	for _, reply := range replies {
		userIds = append(userIds, reply.UserID, reply.AtUserID)
	}
	users, err := getUsersByIDs(s.db, userIds)
	if err != nil {
		return nil, err
	}
	repliesFormated := []*ms.CourseCommentReplyFormated{}
	for _, reply := range replies {
		replyFormated := reply.Format()
		for _, user := range users {
			if reply.UserID == user.ID {
				replyFormated.User = user.Format()
			}
			if reply.AtUserID == user.ID {
				replyFormated.AtUser = user.Format()
			}
		}
		if replyFormated.User == nil {
			// 作者用户已不存在时填充占位 避免前端空指针
			replyFormated.User = ms.GhostUserFormated
		}
		repliesFormated = append(repliesFormated, replyFormated)
	}
	return repliesFormated, nil
}

func (s *courseManageSrv) CreateCourseGroup(g *ms.CourseGroup) (*ms.CourseGroup, error) {
	return g.Create(s.db)
}

func (s *courseManageSrv) UpdateCourseGroup(g *ms.CourseGroup) error {
	return g.Update(s.db)
}

func (s *courseManageSrv) DeleteCourseGroup(id int64) error {
	group := &dbr.CourseGroup{
		Model: &dbr.Model{ID: id},
	}
	// 硬删除(与课程一致 不走软删)
	return group.Delete(s.db)
}

func (s *courseManageSrv) CountCoursesByGroup(groupId int64) (count int64, err error) {
	err = s.db.Model(&dbr.Course{}).Where("group_id = ? AND is_del = ?", groupId, 0).Count(&count).Error
	return
}

func (s *courseManageSrv) CreateCourse(c *ms.Course) (*ms.Course, error) {
	return c.Create(s.db)
}

func (s *courseManageSrv) UpdateCourse(c *ms.Course) error {
	return c.Update(s.db)
}

// DeleteCourse 课程硬删除: 同事务硬删其评论/回复/内容(课程无状态流转 不走软删)
func (s *courseManageSrv) DeleteCourse(course *ms.Course) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var commentIds []int64
		if err := tx.Model(&dbr.CourseComment{}).Unscoped().Where("course_id = ?", course.ID).
			Select("id").Find(&commentIds).Error; err != nil {
			return err
		}
		if len(commentIds) > 0 {
			if err := tx.Unscoped().Where("comment_id IN ?", commentIds).Delete(&dbr.CourseCommentContent{}).Error; err != nil {
				return err
			}
			if err := tx.Unscoped().Where("comment_id IN ?", commentIds).Delete(&dbr.CourseCommentReply{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Unscoped().Where("course_id = ?", course.ID).Delete(&dbr.CourseComment{}).Error; err != nil {
			return err
		}
		return course.DeleteUnscoped(tx)
	})
}

func (s *courseManageSrv) IncrCoursePlayCount(id int64) (count int64, err error) {
	if err = s.db.Model(&dbr.Course{}).Where("id = ? AND is_del = ?", id, 0).
		Update("play_count", gorm.Expr("play_count+1")).Error; err != nil {
		return
	}
	err = s.db.Model(&dbr.Course{}).Where("id = ?", id).Pluck("play_count", &count).Error
	return
}

func (s *courseManageSrv) CreateCourseComment(c *ms.CourseComment) (*ms.CourseComment, error) {
	return c.Create(s.db)
}

func (s *courseManageSrv) CreateCourseCommentContent(c *ms.CourseCommentContent) (*ms.CourseCommentContent, error) {
	return c.Create(s.db)
}

func (s *courseManageSrv) CreateCourseCommentReply(reply *ms.CourseCommentReply) (res *ms.CourseCommentReply, err error) {
	if res, err = reply.Create(s.db); err == nil && reply.AuditStatus == dbr.PostAuditApproved {
		// 仅即时过审的回复计入回复数 待审核的在过审时补记(见UpdateCourseCommentReplyAuditStatus)
		// 宽松处理错误
		s.db.Model(&dbr.CourseComment{}).Where("id = ?", reply.CommentID).Update("reply_count", gorm.Expr("reply_count+1"))
	}
	return
}

func (s *courseManageSrv) DeleteCourseComment(c *ms.CourseComment) error {
	return c.Delete(s.db)
}

func (s *courseManageSrv) DeleteCourseCommentReply(r *ms.CourseCommentReply) error {
	db := s.db.Begin()
	defer db.Rollback()
	if err := r.Delete(db); err != nil {
		return err
	}
	// 仅已过审回复曾计入reply_count 待审/未过审回复删除时不回减
	if r.AuditStatus == dbr.PostAuditApproved {
		// 宽松处理错误
		db.Model(&dbr.CourseComment{}).Where("id = ?", r.CommentID).Update("reply_count", gorm.Expr("reply_count-1"))
	}
	db.Commit()
	return nil
}

// UpdateCourseCommentAuditStatus 更新课程评论审核状态 返回旧状态供上层联动课程评论数/通知
func (s *courseManageSrv) UpdateCourseCommentAuditStatus(id int64, status int) (oldStatus int, err error) {
	var comment dbr.CourseComment
	if err = s.db.Where("id = ? AND is_del = ?", id, 0).First(&comment).Error; err != nil {
		return
	}
	oldStatus = int(comment.AuditStatus)
	err = s.db.Model(&dbr.CourseComment{}).Where("id = ?", id).Updates(map[string]any{
		"audit_status": status,
		"modified_on":  time.Now().Unix(),
	}).Error
	return
}

// UpdateCourseCommentReplyAuditStatus 更新课程回复审核状态 返回旧状态 并联动父评论reply_count
func (s *courseManageSrv) UpdateCourseCommentReplyAuditStatus(id int64, status int) (oldStatus int, err error) {
	var reply dbr.CourseCommentReply
	if err = s.db.Where("id = ? AND is_del = ?", id, 0).First(&reply).Error; err != nil {
		return
	}
	oldStatus = int(reply.AuditStatus)
	if err = s.db.Model(&dbr.CourseCommentReply{}).Where("id = ?", id).Updates(map[string]any{
		"audit_status": status,
		"modified_on":  time.Now().Unix(),
	}).Error; err != nil {
		return
	}
	// 待审/未过审 -> 过审: 补记回复数; 过审 -> 拒绝: 回减
	switch {
	case oldStatus != int(dbr.PostAuditApproved) && status == int(dbr.PostAuditApproved):
		err = s.db.Model(&dbr.CourseComment{}).Where("id = ?", reply.CommentID).Update("reply_count", gorm.Expr("reply_count+1")).Error
	case oldStatus == int(dbr.PostAuditApproved) && status != int(dbr.PostAuditApproved):
		err = s.db.Model(&dbr.CourseComment{}).Where("id = ?", reply.CommentID).Update("reply_count", gorm.Expr("reply_count-1")).Error
	}
	return
}

func (s *courseManageSrv) AdjustCourseCommentCount(courseId int64, delta int) error {
	return s.db.Model(&dbr.Course{}).Where("id = ?", courseId).
		Update("comment_count", gorm.Expr("comment_count+?", delta)).Error
}
