// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package dbr

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PostStar struct {
	*Model
	Post   *Post `json:"-"`
	PostID int64 `json:"post_id"`
	UserID int64 `json:"user_id"`
}

func (p *PostStar) Get(db *gorm.DB) (*PostStar, error) {
	var star PostStar
	tn := db.NamingStrategy.TableName("PostStar") + "."

	if p.Model != nil && p.ID > 0 {
		db = db.Where(tn+"id = ? AND "+tn+"is_del = ?", p.ID, 0)
	}
	if p.PostID > 0 {
		db = db.Where(tn+"post_id = ?", p.PostID)
	}
	if p.UserID > 0 {
		db = db.Where(tn+"user_id = ?", p.UserID)
	}

	db = db.Where(tn+"is_del = ?", 0)
	if err := db.First(&star).Error; err != nil {
		return nil, err
	}
	return &star, nil
}

func (p *PostStar) Create(db *gorm.DB) (*PostStar, error) {
	err := db.Omit("Post").Create(&p).Error

	return p, err
}

func (p *PostStar) Delete(db *gorm.DB) error {
	return db.Model(&PostStar{}).Omit("Post").Where("id = ? AND is_del = ?", p.Model.ID, 0).Updates(map[string]any{
		"deleted_on": time.Now().Unix(),
		"is_del":     1,
	}).Error
}

// canViewPostCond mirrors the current permission-based CanViewTweet policy.
// Viewer identity is independent of the owner of the starred-post list.
func canViewPostCond(db *gorm.DB, viewer *User) (string, []any) {
	col := func(name string) clause.Column { return clause.Column{Table: "Post", Name: name} }
	if viewer == nil || viewer.Model == nil || viewer.ID <= 0 {
		return "? = ? AND ? = ?", []any{col("audit_status"), PostAuditApproved, col("visibility"), PostVisitPublic}
	}
	if !viewer.HasPermission("post.view") {
		return "1 = 0", nil
	}
	following := db.Session(&gorm.Session{NewDB: true}).Unscoped().Model(&Following{}).Select("1").Where("user_id = ? AND follow_id = ?", viewer.ID, col("user_id"))
	approved := "(? = ? AND (? = ? OR ? OR (? = ? AND EXISTS (?))))"
	args := []any{col("user_id"), viewer.ID, col("audit_status"), PostAuditApproved, col("visibility"), PostVisitPublic, viewer.HasPermission("content.view_private"), col("visibility"), PostVisitFollowing, following}
	sql := "(? = ? OR " + approved
	if viewer.HasPermission("audit.view_all") {
		sql += " OR ? <> ?"
		args = append(args, col("audit_status"), PostAuditApproved)
	} else if viewer.HasPermission("content.review") {
		assigned := db.Session(&gorm.Session{NewDB: true}).Model(&ReviewTask{}).Select("1").Where("kind = ? AND target_id = ? AND assignee_id = ?", ReviewPost, col("id"), viewer.ID)
		sql += " OR (? <> ? AND EXISTS (?))"
		args = append(args, col("audit_status"), PostAuditApproved, assigned)
	}
	return sql + ")", args
}

func scopeReactionPostVisibility(db *gorm.DB, viewerID int64) *gorm.DB {
	viewer := &User{Model: &Model{ID: viewerID}, Status: UserStatusNormal, Permissions: []string{"post.view"}}
	condition, args := canViewPostCond(db, viewer)
	return db.Where(clause.Eq{Column: clause.Column{Table: "Post", Name: "is_del"}, Value: 0}).Where(condition, args...)
}

func (p *PostStar) List(db *gorm.DB, conditions *ConditionsT, viewer *User, limit, offset int) (res []*PostStar, err error) {
	tn := db.NamingStrategy.TableName("PostStar") + "."
	if offset >= 0 && limit > 0 {
		db = db.Offset(offset).Limit(limit)
	}
	if p.UserID > 0 {
		db = db.Where(tn+"user_id = ?", p.UserID)
	}
	for k, v := range *conditions {
		if k == "ORDER" {
			db = db.Order(v)
		} else {
			db = db.Where(tn+k, v)
		}
	}
	db = db.Joins("Post").Where(clause.Eq{Column: clause.Column{Table: "Post", Name: "is_del"}, Value: 0})
	if cond, args := canViewPostCond(db, viewer); cond != "" {
		db = db.Where(cond, args...)
	}
	db = db.Order(clause.OrderByColumn{Column: clause.Column{Table: "Post", Name: "id"}, Desc: true})
	err = db.Find(&res).Error
	return
}

func (p *PostStar) Count(db *gorm.DB, viewer *User, conditions *ConditionsT) (res int64, err error) {
	tn := db.NamingStrategy.TableName("PostStar") + "."
	if p.PostID > 0 {
		db = db.Where(tn+"post_id = ?", p.PostID)
	}
	if p.UserID > 0 {
		db = db.Where(tn+"user_id = ?", p.UserID)
	}
	for k, v := range *conditions {
		if k != "ORDER" {
			db = db.Where(tn+k, v)
		}
	}
	db = db.Joins("Post").Where(clause.Eq{Column: clause.Column{Table: "Post", Name: "is_del"}, Value: 0})
	if cond, args := canViewPostCond(db, viewer); cond != "" {
		db = db.Where(cond, args...)
	}
	err = db.Model(p).Count(&res).Error
	return
}
