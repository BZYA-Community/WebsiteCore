// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package dbr

import (
	"time"

	"github.com/BZYA-Community/WebsiteCore/internal/core/cs"
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

func (p *PostStar) List(db *gorm.DB, conditions *ConditionsT, typ cs.RelationTyp, limit int, offset int) (res []*PostStar, err error) {
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
	db = scopeReactionPostVisibility(db.Joins("Post"), p.UserID, typ)
	db = db.Order(clause.OrderByColumn{Column: clause.Column{Table: "Post", Name: "id"}, Desc: true})
	err = db.Find(&res).Error
	return
}

func (p *PostStar) Count(db *gorm.DB, typ cs.RelationTyp, conditions *ConditionsT) (res int64, err error) {
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
	db = scopeReactionPostVisibility(db.Joins("Post"), p.UserID, typ)
	err = db.Model(p).Count(&res).Error
	return
}

// scopeReactionPostVisibility 按访问关系收敛点赞/收藏列表的帖子读口径(安全审计02 中-1/低-2),
// 与 application/content/views.go 的 CanViewTweet 保持一致:
//   - 管理侧: 不过滤(保持原有全量语义);
//   - 本人: 公开帖 + 自己的任意可见性帖 + 仍关注作者的关注可见帖, 且他人的未过审帖一律不可见
//     (修复取关后关注可见帖仍残留在自己点赞/收藏列表的问题);
//   - 游客/他人: 仅已过审的公开帖(修复"作者自赞 + 公开点赞列表"绕过审核泄露待审帖的问题)。
func scopeReactionPostVisibility(db *gorm.DB, viewerID int64, typ cs.RelationTyp) *gorm.DB {
	postUserID := clause.Column{Table: "Post", Name: "user_id"}
	postAuditStatus := clause.Column{Table: "Post", Name: "audit_status"}
	switch typ {
	case cs.RelationAdmin:
		// admin have all permition to visit all type tweets
	case cs.RelationSelf:
		followIDs := db.Session(&gorm.Session{NewDB: true}).Unscoped().Model(&Following{}).
			Select("follow_id").Where("user_id = ?", viewerID)
		db = db.Where("? = ? OR ? = ?", postAuditStatus, PostAuditApproved, postUserID, viewerID).
			Where("visibility = ? OR ? = ? OR (visibility = ? AND ? IN (?))",
				PostVisitPublic, postUserID, viewerID, PostVisitFollowing, postUserID, followIDs)
	default:
		db = db.Where("visibility = ? AND ? = ?", PostVisitPublic, postAuditStatus, PostAuditApproved)
	}
	return db
}
