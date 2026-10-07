// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package jinzhu

import (
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
		groups, listErr := (&dbr.CourseGroup{}).List(s.db)
		if listErr != nil {
			return nil, 0, listErr
		}
		children := make(map[int64][]int64, len(groups))
		for _, group := range groups {
			children[group.ParentID] = append(children[group.ParentID], group.ID)
		}
		ids, seen := []int64{groupId}, map[int64]bool{groupId: true}
		for i := 0; i < len(ids); i++ {
			for _, child := range children[ids[i]] {
				if !seen[child] {
					seen[child] = true
					ids = append(ids, child)
				}
			}
		}
		db = db.Where("group_id IN ?", ids)
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

func (s *courseManageSrv) CountCoursesByGroup(groupId int64) (count int64, err error) {
	err = s.db.Model(&dbr.Course{}).Where("group_id = ? AND is_del = ?", groupId, 0).Count(&count).Error
	return
}

// DeleteCourse 课程硬删除: 同事务硬删其评论/回复/内容(课程无状态流转 不走软删)
func (s *courseManageSrv) DeleteCourse(actor *ms.User, course *ms.Course) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		fresh, err := lockManagedCourse(tx, actor, course.ID)
		if err != nil {
			return err
		}
		var commentIds []int64
		if err := tx.Model(&dbr.CourseComment{}).Unscoped().Where("course_id = ?", course.ID).
			Select("id").Find(&commentIds).Error; err != nil {
			return err
		}
		if len(commentIds) > 0 {
			var answerIDs []int64
			if err := tx.Model(&dbr.CourseCommentReply{}).Unscoped().Where("comment_id IN ?", commentIds).Pluck("id", &answerIDs).Error; err != nil {
				return err
			}
			if err := cancelTargetReviewsTx(tx, ms.ReviewCourseQuestion, commentIds); err != nil {
				return err
			}
			if err := cancelTargetReviewsTx(tx, ms.ReviewCourseAnswer, answerIDs); err != nil {
				return err
			}
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
		lessonIDs := tx.Model(&dbr.CourseLesson{}).Unscoped().Select("id").Where("course_id = ?", course.ID)
		if err := tx.Where("lesson_id IN (?)", lessonIDs).Delete(&dbr.CourseLessonAttachment{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("course_id = ?", course.ID).Delete(&dbr.CourseLesson{}).Error; err != nil {
			return err
		}
		if err := course.DeleteUnscoped(tx); err != nil {
			return err
		}
		return dbr.AppendOperationLog(tx, actor.ID, "delete", "course", course.ID, fresh.Format(), nil)
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

func (s *courseManageSrv) AdjustCourseCommentCount(courseId int64, delta int) error {
	return s.db.Model(&dbr.Course{}).Where("id = ?", courseId).
		Update("comment_count", gorm.Expr("comment_count+?", delta)).Error
}
