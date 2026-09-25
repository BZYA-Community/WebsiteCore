// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package dbr

import (
	"time"

	"gorm.io/gorm"
)

// CourseGroup 课程分组(管理员动态维护)
type CourseGroup struct {
	*Model
	Name string `json:"name"`
	Sort int    `json:"sort"`
}

// CourseGroupFormated 课程分组(含课程数)
type CourseGroupFormated struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Sort        int    `json:"sort"`
	CourseCount int64  `json:"course_count"`
}

func (g *CourseGroup) Format(courseCount int64) *CourseGroupFormated {
	if g.Model == nil {
		return &CourseGroupFormated{}
	}
	return &CourseGroupFormated{
		ID:          g.Model.ID,
		Name:        g.Name,
		Sort:        g.Sort,
		CourseCount: courseCount,
	}
}

func (g *CourseGroup) Create(db *gorm.DB) (*CourseGroup, error) {
	err := db.Create(&g).Error
	return g, err
}

func (g *CourseGroup) Get(db *gorm.DB) (*CourseGroup, error) {
	var group CourseGroup
	if g.Model == nil || g.ID <= 0 {
		return nil, gorm.ErrRecordNotFound
	}
	err := db.Where("id = ? AND is_del = ?", g.ID, 0).First(&group).Error
	if err != nil {
		return &group, err
	}
	return &group, nil
}

func (g *CourseGroup) List(db *gorm.DB) ([]*CourseGroup, error) {
	var groups []*CourseGroup
	err := db.Where("is_del = ?", 0).Order("sort, id").Find(&groups).Error
	return groups, err
}

func (g *CourseGroup) Update(db *gorm.DB) error {
	return db.Model(g).Where("id = ?", g.Model.ID).Updates(map[string]any{
		"name": g.Name,
		"sort": g.Sort,
	}).Error
}

// Delete 分组硬删除(调用方需保证分组下无课程)
func (g *CourseGroup) Delete(db *gorm.DB) error {
	return db.Unscoped().Where("id = ?", g.Model.ID).Delete(&CourseGroup{}).Error
}

// Course 课程: 无状态流转, 创建即上线; 删除为硬删除
type Course struct {
	*Model
	GroupID      int64  `json:"group_id"`
	TeacherID    int64  `json:"teacher_id"`
	Title        string `json:"title"`
	Intro        string `json:"intro"`
	VideoURL     string `json:"video_url"`
	Cover        string `json:"cover"`
	PlayCount    int64  `json:"play_count"`
	CommentCount int64  `json:"comment_count"`
}

// CourseFormated 课程输出(含分组名与老师信息)
type CourseFormated struct {
	ID           int64         `json:"id"`
	GroupID      int64         `json:"group_id"`
	GroupName    string        `json:"group_name"`
	TeacherID    int64         `json:"teacher_id"`
	Teacher      *UserFormated `json:"teacher"`
	Title        string        `json:"title"`
	Intro        string        `json:"intro"`
	Cover        string        `json:"cover"`
	PlayCount    int64         `json:"play_count"`
	CommentCount int64         `json:"comment_count"`
	CreatedOn    int64         `json:"created_on"`
}

func (c *Course) Format() *CourseFormated {
	if c.Model == nil {
		return &CourseFormated{}
	}
	return &CourseFormated{
		ID:           c.Model.ID,
		GroupID:      c.GroupID,
		TeacherID:    c.TeacherID,
		Teacher:      &UserFormated{},
		Title:        c.Title,
		Intro:        c.Intro,
		Cover:        c.Cover,
		PlayCount:    c.PlayCount,
		CommentCount: c.CommentCount,
		CreatedOn:    c.CreatedOn,
	}
}

func (c *Course) Create(db *gorm.DB) (*Course, error) {
	err := db.Create(&c).Error
	return c, err
}

func (c *Course) Get(db *gorm.DB) (*Course, error) {
	var course Course
	if c.Model == nil || c.ID <= 0 {
		return nil, gorm.ErrRecordNotFound
	}
	err := db.Where("id = ? AND is_del = ?", c.ID, 0).First(&course).Error
	if err != nil {
		return &course, err
	}
	return &course, nil
}

func (c *Course) Update(db *gorm.DB) error {
	return db.Model(c).Where("id = ?", c.Model.ID).Updates(map[string]any{
		"group_id":   c.GroupID,
		"teacher_id": c.TeacherID,
		"title":      c.Title,
		"intro":      c.Intro,
		"video_url":  c.VideoURL,
		"cover":      c.Cover,
	}).Error
}

// DeleteUnscoped 课程硬删除(评论等关联数据由调用方在同一事务内清理)
func (c *Course) DeleteUnscoped(db *gorm.DB) error {
	return db.Unscoped().Where("id = ?", c.Model.ID).Delete(&Course{}).Error
}

// CourseComment 课程评论: 审核口径与帖子评论一致
type CourseComment struct {
	*Model
	CourseID    int64      `json:"course_id"`
	UserID      int64      `json:"user_id"`
	IP          string     `json:"ip"`
	IPLoc       string     `json:"ip_loc"`
	ReplyCount  int32      `json:"reply_count"`
	AuditStatus PostAuditT `json:"audit_status"`
}

type CourseCommentFormated struct {
	ID          int64                         `json:"id"`
	CourseID    int64                         `json:"course_id"`
	UserID      int64                         `json:"user_id"`
	User        *UserFormated                 `json:"user"`
	Contents    []*CourseCommentContent       `json:"contents"`
	Replies     []*CourseCommentReplyFormated `json:"replies"`
	IPLoc       string                        `json:"ip_loc"`
	ReplyCount  int32                         `json:"reply_count"`
	AuditStatus PostAuditT                    `json:"audit_status"`
	CreatedOn   int64                         `json:"created_on"`
	ModifiedOn  int64                         `json:"modified_on"`
}

func (c *CourseComment) Format() *CourseCommentFormated {
	if c.Model == nil {
		return &CourseCommentFormated{}
	}
	return &CourseCommentFormated{
		ID:          c.Model.ID,
		CourseID:    c.CourseID,
		UserID:      c.UserID,
		User:        &UserFormated{},
		Contents:    []*CourseCommentContent{},
		Replies:     []*CourseCommentReplyFormated{},
		IPLoc:       c.IPLoc,
		ReplyCount:  c.ReplyCount,
		AuditStatus: c.AuditStatus,
		CreatedOn:   c.CreatedOn,
		ModifiedOn:  c.ModifiedOn,
	}
}

func (c *CourseComment) Create(db *gorm.DB) (*CourseComment, error) {
	err := db.Create(&c).Error
	return c, err
}

func (c *CourseComment) Get(db *gorm.DB) (*CourseComment, error) {
	var comment CourseComment
	if c.Model == nil || c.ID <= 0 {
		return nil, gorm.ErrRecordNotFound
	}
	err := db.Where("id = ? AND is_del = ?", c.ID, 0).First(&comment).Error
	if err != nil {
		return &comment, err
	}
	return &comment, nil
}

// Delete 用户删除评论(软删, 与帖子评论一致)
func (c *CourseComment) Delete(db *gorm.DB) error {
	return db.Model(c).Where("id = ?", c.Model.ID).Updates(map[string]any{
		"deleted_on": time.Now().Unix(),
		"is_del":     1,
	}).Error
}

// CourseCommentContent 课程评论内容(分块)
type CourseCommentContent struct {
	*Model
	CommentID int64        `json:"comment_id"`
	UserID    int64        `json:"user_id"`
	Content   string       `json:"content"`
	Type      PostContentT `json:"type"`
	Sort      int64        `json:"sort"`
}

func (c *CourseCommentContent) Create(db *gorm.DB) (*CourseCommentContent, error) {
	err := db.Create(&c).Error
	return c, err
}

// CourseCommentReply 课程评论回复(楼中楼)
type CourseCommentReply struct {
	*Model
	CommentID   int64      `json:"comment_id"`
	UserID      int64      `json:"user_id"`
	AtUserID    int64      `json:"at_user_id"`
	Content     string     `json:"content"`
	IP          string     `json:"ip"`
	IPLoc       string     `json:"ip_loc"`
	AuditStatus PostAuditT `json:"audit_status"`
}

type CourseCommentReplyFormated struct {
	ID          int64         `json:"id"`
	CommentID   int64         `json:"comment_id"`
	UserID      int64         `json:"user_id"`
	User        *UserFormated `json:"user"`
	AtUserID    int64         `json:"at_user_id"`
	AtUser      *UserFormated `json:"at_user"`
	Content     string        `json:"content"`
	IPLoc       string        `json:"ip_loc"`
	AuditStatus PostAuditT    `json:"audit_status"`
	CreatedOn   int64         `json:"created_on"`
	ModifiedOn  int64         `json:"modified_on"`
}

func (c *CourseCommentReply) Format() *CourseCommentReplyFormated {
	if c.Model == nil {
		return &CourseCommentReplyFormated{}
	}
	return &CourseCommentReplyFormated{
		ID:          c.ID,
		CommentID:   c.CommentID,
		UserID:      c.UserID,
		User:        &UserFormated{},
		AtUserID:    c.AtUserID,
		AtUser:      &UserFormated{},
		Content:     c.Content,
		IPLoc:       c.IPLoc,
		AuditStatus: c.AuditStatus,
		CreatedOn:   c.CreatedOn,
		ModifiedOn:  c.ModifiedOn,
	}
}

func (c *CourseCommentReply) Create(db *gorm.DB) (*CourseCommentReply, error) {
	err := db.Create(&c).Error
	return c, err
}

func (c *CourseCommentReply) Get(db *gorm.DB) (*CourseCommentReply, error) {
	var reply CourseCommentReply
	if c.Model == nil || c.ID <= 0 {
		return nil, gorm.ErrRecordNotFound
	}
	err := db.Where("id = ? AND is_del = ?", c.ID, 0).First(&reply).Error
	if err != nil {
		return &reply, err
	}
	return &reply, nil
}

// Delete 用户删除回复(软删, 与帖子回复一致)
func (c *CourseCommentReply) Delete(db *gorm.DB) error {
	return db.Model(c).Where("id = ? AND is_del = ?", c.Model.ID, 0).Updates(map[string]any{
		"deleted_on": time.Now().Unix(),
		"is_del":     1,
	}).Error
}
