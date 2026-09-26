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

	db = db.Joins("Post").Where("visibility <> ? OR (visibility = ? AND ? = ?)", PostVisitPrivate, PostVisitPrivate, clause.Column{Table: "Post", Name: "user_id"}, p.UserID).Order(clause.OrderByColumn{Column: clause.Column{Table: "Post", Name: "id"}, Desc: true})
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

// canViewPostCond 构造星标列表的可见性谓词, 语义与 servants/base.CanViewTweet 的读口径同源
// (dao 层不能 import servants, 因此在 SQL 层镜像同一套判定, 避免出现第 4 套实现):
//   - 管理员/审核员: 与 CanViewTweet 的管理侧豁免一致, 整表放行(返回空条件);
//   - 帖子作者: 按行放行(post.user_id = 访问者);
//   - 其余访问者(含游客): 要求帖子已过审(audit_status=approved),
//     且可见性为公开, 或关注可见且访问者已关注帖子作者(镜像 Ds.IsFollow 的行存在性);
//   - 私密(0)与好友可见(50)对非作者/非管理侧一律拒绝 —— 好友功能已移除(库中已无好友关系表),
//     与 CanViewTweet 的 default 分支、getUserTweets 的"好友可见按私密口径"保持一致。
//
// viewer 为 nil 或 ID<=0 时按游客处理; 返回空串表示无需附加条件。
func canViewPostCond(db *gorm.DB, viewer *User) (string, []any) {
	if viewer != nil && viewer.ID > 0 && (viewer.IsAdmin || viewer.HasRole(RoleAuditor)) {
		return "", nil
	}
	var viewerID int64
	if viewer != nil {
		viewerID = viewer.ID
	}
	postCol := func(name string) clause.Column {
		return clause.Column{Table: "Post", Name: name}
	}
	sql := "((? > 0 AND ? = ?) OR " +
		"(? = ? AND (? = ? OR (? = ? AND EXISTS (SELECT 1 FROM " +
		db.NamingStrategy.TableName("Following") +
		" WHERE user_id = ? AND follow_id = ?)))))"
	args := []any{
		viewerID, postCol("user_id"), viewerID,
		postCol("audit_status"), PostAuditApproved,
		postCol("visibility"), PostVisitPublic,
		postCol("visibility"), PostVisitFollowing,
		viewerID, postCol("user_id"),
	}
	return sql, args
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
	db = db.Joins("Post")
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
	db = db.Joins("Post")
	if cond, args := canViewPostCond(db, viewer); cond != "" {
		db = db.Where(cond, args...)
	}
	err = db.Model(p).Count(&res).Error
	return
}
