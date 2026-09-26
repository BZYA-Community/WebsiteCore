// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package jinzhu

import (
	"fmt"
	"time"

	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/core/cs"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/jinzhu/dbr"
	"github.com/BZYA-Community/WebsiteCore/pkg/types"
	"gorm.io/gorm"
)

type commentSrv struct {
	db *gorm.DB
}

type commentManageSrv struct {
	db *gorm.DB
}

func newCommentService(db *gorm.DB) core.CommentService {
	return &commentSrv{
		db: db,
	}
}

func newCommentManageService(db *gorm.DB) core.CommentManageService {
	return &commentManageSrv{
		db: db,
	}
}

func (s *commentSrv) GetCommentThumbsMap(userId int64, tweetId int64) (cs.CommentThumbsMap, cs.CommentThumbsMap, error) {
	if userId < 0 {
		return nil, nil, nil
	}
	commentThumbsList := cs.CommentThumbsList{}
	err := s.db.Model(&dbr.TweetCommentThumbs{}).Where("user_id=? AND tweet_id=?", userId, tweetId).Find(&commentThumbsList).Error
	if err != nil {
		return nil, nil, err
	}
	commentThumbs, replyThumbs := make(cs.CommentThumbsMap), make(cs.CommentThumbsMap)
	for _, thumbs := range commentThumbsList {
		if thumbs.CommentType == 0 {
			commentThumbs[thumbs.CommentID] = thumbs
		} else {
			replyThumbs[thumbs.ReplyID] = thumbs
		}
	}
	return commentThumbs, replyThumbs, nil
}

// addCommentAuditScope 评论审核可见范围(评论与回复共用):
// 审核员/管理员见全部; 登录用户见已过审+本人发布的任意状态; 游客仅见已过审
func addCommentAuditScope(db *gorm.DB, viewerId int64, viewerIsAuditor bool) *gorm.DB {
	switch {
	case viewerIsAuditor:
		return db
	case viewerId > 0:
		return db.Where("(audit_status = ? OR user_id = ?)", int(dbr.PostAuditApproved), viewerId)
	default:
		return db.Where("audit_status = ?", int(dbr.PostAuditApproved))
	}
}

func (s *commentSrv) GetComments(tweetId int64, style cs.StyleCommentType, viewerId int64, viewerIsAuditor bool, limit int, offset int) (res []*ms.Comment, total int64, err error) {
	db := s.db.Table(_comment_)
	sort := "is_essence DESC, id ASC"
	switch style {
	case cs.StyleCommentHots:
		// rank_score=评论回复数*2+点赞*4-点踩, order byrank_score DESC
		db = db.Joins(fmt.Sprintf("LEFT JOIN %s m ON %s.id=m.comment_id AND m.is_del=0", _commentMetric_, _comment_))
		sort = fmt.Sprintf("is_essence DESC, m.rank_score DESC, %s.id DESC", _comment_)
	case cs.StyleCommentNewest:
		sort = "is_essence DESC, id DESC"
	case cs.StyleCommentDefault:
		fallthrough
	default:
		// nothing
	}
	db = db.Where("post_id=?", tweetId)
	db = addCommentAuditScope(db, viewerId, viewerIsAuditor)
	if err = db.Count(&total).Error; err != nil {
		return
	}
	err = db.Order(sort).Limit(limit).Offset(offset).Find(&res).Error
	return
}

func (s *commentSrv) GetCommentByID(id int64) (*ms.Comment, error) {
	comment := &dbr.Comment{
		Model: &dbr.Model{
			ID: id,
		},
	}
	return comment.Get(s.db)
}

func (s *commentSrv) GetCommentReplyByID(id int64) (*ms.CommentReply, error) {
	reply := &dbr.CommentReply{
		Model: &dbr.Model{
			ID: id,
		},
	}
	return reply.Get(s.db)
}

// GetCommentRepliesByReplyIDs 批量按回复id取回复(软删除的不返回 缺失的id不在返回中)
func (s *commentSrv) GetCommentRepliesByReplyIDs(ids []int64) ([]*ms.CommentReply, error) {
	if len(ids) == 0 {
		return []*ms.CommentReply{}, nil
	}
	return (&dbr.CommentReply{}).List(s.db, &dbr.ConditionsT{
		"id IN ?": ids,
	}, 0, 0)
}

func (s *commentSrv) GetCommentCount(conditions *ms.ConditionsT) (int64, error) {
	return (&dbr.Comment{}).Count(s.db, conditions)
}

func (s *commentSrv) GetCommentContentsByIDs(ids []int64) ([]*ms.CommentContent, error) {
	commentContent := &dbr.CommentContent{}
	return commentContent.List(s.db, &dbr.ConditionsT{
		"comment_id IN ?": ids,
	}, 0, 0)
}

func (s *commentSrv) GetCommentRepliesByID(ids []int64, viewerId int64, viewerIsAuditor bool) ([]*ms.CommentReplyFormated, error) {
	CommentReply := &dbr.CommentReply{}
	replies, err := CommentReply.List(s.db, &dbr.ConditionsT{
		"comment_id IN ?": ids,
		"ORDER":           "id ASC",
	}, 0, 0)
	if err != nil {
		return nil, err
	}
	// 审核可见范围与评论同口径(回复量以页内评论为界 内存过滤即可)
	if !viewerIsAuditor {
		kept := make([]*dbr.CommentReply, 0, len(replies))
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
	repliesFormated := []*ms.CommentReplyFormated{}
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

func (s *commentManageSrv) HighlightComment(userId, commentId int64) (isEssence int8, err error) {
	post := &dbr.Post{}
	comment := &dbr.Comment{}
	db := s.db.Model(comment)
	if err = db.Where("id=?", commentId).First(comment).Error; err != nil {
		return
	}
	if err = s.db.Table(_post_).Where("id=?", comment.PostID).First(post).Error; err != nil {
		return
	}
	if post.UserID != userId {
		return 0, cs.ErrNoPermission
	}
	isEssence = 1 - comment.IsEssence
	err = s.db.Model(comment).UpdateColumns(map[string]any{
		"is_essence":  isEssence,
		"modified_on": time.Now().Unix(),
	}).Where("id=?", commentId).Error
	return
}

func (s *commentManageSrv) DeleteComment(comment *ms.Comment) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := comment.Delete(tx); err != nil {
			return err
		}
		return tx.Model(&dbr.TweetCommentThumbs{}).Where("user_id=? AND tweet_id=? AND comment_id=?", comment.UserID, comment.PostID, comment.ID).Updates(map[string]any{
			"deleted_on": time.Now().Unix(),
			"is_del":     1,
		}).Error
	})
}

func (s *commentManageSrv) CreateComment(comment *ms.Comment) (*ms.Comment, error) {
	return comment.Create(s.db)
}

func (s *commentManageSrv) CreateCommentReply(reply *ms.CommentReply) (res *ms.CommentReply, err error) {
	if res, err = reply.Create(s.db); err == nil && reply.AuditStatus == dbr.PostAuditApproved {
		// 仅即时过审的回复计入回复数 待审核的在过审时补记(见UpdateCommentReplyAuditStatus)
		// 宽松处理错误
		s.db.Table(_comment_).Where("id=?", reply.CommentID).Update("reply_count", gorm.Expr("reply_count+1"))
	}
	return
}

func (s *commentManageSrv) DeleteCommentReply(reply *ms.CommentReply) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := reply.Delete(tx); err != nil {
			return err
		}
		if err := tx.Model(&dbr.TweetCommentThumbs{}).
			Where("user_id=? AND comment_id=? AND reply_id=?", reply.UserID, reply.CommentID, reply.ID).Updates(map[string]any{
			"deleted_on": time.Now().Unix(),
			"is_del":     1,
		}).Error; err != nil {
			return err
		}
		// 仅已过审回复曾计入reply_count 待审/未过审回复删除时不回减
		if reply.AuditStatus == dbr.PostAuditApproved {
			// 宽松处理错误
			tx.Table(_comment_).Where("id=?", reply.CommentID).Update("reply_count", gorm.Expr("reply_count-1"))
		}
		return nil
	})
}

func (s *commentManageSrv) CreateCommentContent(content *ms.CommentContent) (*ms.CommentContent, error) {
	return content.Create(s.db)
}

// thumbsDirection 点赞方向
type thumbsDirection int

const (
	// thumbsUp 顶赞
	thumbsUp thumbsDirection = iota
	// thumbsDown 点踩
	thumbsDown
)

// thumbsTarget 点赞目标: 评论或回复
type thumbsTarget struct {
	id          int64 // 计数所在行id: 评论场景为commentId 回复场景为replyId
	commentType int8  // tweet_comment_thumbs.comment_type: 0评论 1回复(为1时查询条件带reply_id)
	countModel  any   // 计数所在表模型: &dbr.Comment{} 或 &dbr.CommentReply{}
}

// toggleThumbs 评论/回复点赞的公共骨架:
// 查旧赞态 -> 按方向计算计数增量与新赞态 -> 保存赞态 -> 事务内原子增减计数
// ThumbsUp/ThumbsDown × Comment/Reply 四个入口的差异全部由 target 与 dir 参数表达 导出签名不变
func (s *commentManageSrv) toggleThumbs(userId, tweetId, commentId int64, target thumbsTarget, dir thumbsDirection) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var (
			thumbsUpCount   int32
			thumbsDownCount int32
		)
		// 查询条件: 回复多带reply_id 并以comment_type区分目标表
		where := "user_id=? AND tweet_id=? AND comment_id=?"
		args := []any{userId, tweetId, commentId}
		if target.commentType == 1 {
			where += " AND reply_id=?"
			args = append(args, target.id)
		}
		where += " AND comment_type=?"
		args = append(args, target.commentType)

		commentThumbs := &dbr.TweetCommentThumbs{}
		// 检查thumbs状态
		err := tx.Where(where, args...).Take(commentThumbs).Error
		if err == nil {
			switch dir {
			case thumbsUp:
				switch {
				// 历史语义差异(仅"同时赞踩"的异常数据下取值不同 此处保留原语义):
				// 评论首分支要求当前未点踩 回复首分支只看IsThumbsUp
				case commentThumbs.IsThumbsUp == types.Yes && (target.commentType != 0 || commentThumbs.IsThumbsDown == types.No):
					thumbsUpCount, thumbsDownCount = -1, 0
				case commentThumbs.IsThumbsUp == types.No && commentThumbs.IsThumbsDown == types.No:
					thumbsUpCount, thumbsDownCount = 1, 0
				default:
					thumbsUpCount, thumbsDownCount = 1, -1
					commentThumbs.IsThumbsDown = types.No
				}
				commentThumbs.IsThumbsUp = 1 - commentThumbs.IsThumbsUp
			case thumbsDown:
				switch {
				case commentThumbs.IsThumbsDown == types.Yes:
					thumbsUpCount, thumbsDownCount = 0, -1
				case commentThumbs.IsThumbsDown == types.No && commentThumbs.IsThumbsUp == types.No:
					thumbsUpCount, thumbsDownCount = 0, 1
				default:
					thumbsUpCount, thumbsDownCount = -1, 1
					commentThumbs.IsThumbsUp = types.No
				}
				commentThumbs.IsThumbsDown = 1 - commentThumbs.IsThumbsDown
			}
			commentThumbs.ModifiedOn = time.Now().Unix()
		} else {
			commentThumbs = &dbr.TweetCommentThumbs{
				UserID:       userId,
				TweetID:      tweetId,
				CommentID:    commentId,
				CommentType:  target.commentType,
				IsThumbsUp:   types.No,
				IsThumbsDown: types.No,
				Model: &dbr.Model{
					CreatedOn: time.Now().Unix(),
				},
			}
			if target.commentType == 1 {
				commentThumbs.ReplyID = target.id
			}
			switch dir {
			case thumbsUp:
				commentThumbs.IsThumbsUp = types.Yes
				thumbsUpCount, thumbsDownCount = 1, 0
			case thumbsDown:
				commentThumbs.IsThumbsDown = types.Yes
				thumbsUpCount, thumbsDownCount = 0, 1
			}
		}
		// 更新thumbs状态
		if err = tx.Save(commentThumbs).Error; err != nil {
			return err
		}
		// 更新thumbsUpCount
		return updateCommentThumbsUpCount(tx, target.countModel, target.id, thumbsUpCount, thumbsDownCount)
	})
}

// ThumbsUpComment 评论点赞
func (s *commentManageSrv) ThumbsUpComment(userId int64, tweetId, commentId int64) error {
	return s.toggleThumbs(userId, tweetId, commentId, thumbsTarget{
		id:          commentId,
		commentType: 0,
		countModel:  &dbr.Comment{},
	}, thumbsUp)
}

// ThumbsDownComment 评论点踩
func (s *commentManageSrv) ThumbsDownComment(userId int64, tweetId, commentId int64) error {
	return s.toggleThumbs(userId, tweetId, commentId, thumbsTarget{
		id:          commentId,
		commentType: 0,
		countModel:  &dbr.Comment{},
	}, thumbsDown)
}

// ThumbsUpReply 回复点赞
func (s *commentManageSrv) ThumbsUpReply(userId int64, tweetId, commentId, replyId int64) error {
	return s.toggleThumbs(userId, tweetId, commentId, thumbsTarget{
		id:          replyId,
		commentType: 1,
		countModel:  &dbr.CommentReply{},
	}, thumbsUp)
}

// ThumbsDownReply 回复点踩
func (s *commentManageSrv) ThumbsDownReply(userId int64, tweetId, commentId, replyId int64) error {
	return s.toggleThumbs(userId, tweetId, commentId, thumbsTarget{
		id:          replyId,
		commentType: 1,
		countModel:  &dbr.CommentReply{},
	}, thumbsDown)
}
func (s *commentManageSrv) updateCommentThumbsUpCount(obj any, id int64, thumbsUpCount, thumbsDownCount int32) error {
	return updateCommentThumbsUpCount(s.db, obj, id, thumbsUpCount, thumbsDownCount)
}
