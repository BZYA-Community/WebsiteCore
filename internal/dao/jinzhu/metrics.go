// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package jinzhu

import (
	"time"

	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/core/cs"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/jinzhu/dbr"
	"gorm.io/gorm"
)

type tweetMetricSrvA struct {
	db *gorm.DB
}

type commentMetricSrvA struct {
	db *gorm.DB
}

type userMetricSrvA struct {
	db *gorm.DB
}

func (s *tweetMetricSrvA) UpdateTweetMetric(metric *cs.TweetMetric) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		postMetric := &dbr.PostMetric{PostId: metric.PostId}
		tx.Model(postMetric).Where("post_id=?", metric.PostId).First(postMetric)
		postMetric.RankScore = metric.RankScore(postMetric.MotivationFactor)
		return tx.Save(postMetric).Error
	})
}

func (s *tweetMetricSrvA) AddTweetMetric(postId int64) (err error) {
	_, err = (&dbr.PostMetric{PostId: postId}).Create(s.db)
	return
}

func (s *tweetMetricSrvA) DeleteTweetMetric(postId int64) (err error) {
	return (&dbr.PostMetric{PostId: postId}).Delete(s.db)
}

func (s *commentMetricSrvA) UpdateCommentMetric(metric *cs.CommentMetric) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		commentMetric := &dbr.CommentMetric{CommentId: metric.CommentId}
		tx.Model(commentMetric).Where("comment_id=?", metric.CommentId).First(commentMetric)
		commentMetric.RankScore = metric.RankScore(commentMetric.MotivationFactor)
		return tx.Save(commentMetric).Error
	})
}

func (s *commentMetricSrvA) AddCommentMetric(commentId int64) (err error) {
	_, err = (&dbr.CommentMetric{CommentId: commentId}).Create(s.db)
	return
}

func (s *commentMetricSrvA) DeleteCommentMetric(commentId int64) (err error) {
	return (&dbr.CommentMetric{CommentId: commentId}).Delete(s.db)
}

// UpdateUserMetric 原子更新用户动态指标 使用SQL表达式就地增减
// 避免"读→改→写"在并发下丢计数
func (s *userMetricSrvA) UpdateUserMetric(userId int64, action uint8) error {
	now := time.Now().Unix()
	// 先查记录是否存在 仅用于决定补建与否 计数增减本身由SQL表达式原子完成
	metric := &dbr.UserMetric{}
	if err := s.db.Model(&dbr.UserMetric{}).Where("user_id = ?", userId).First(metric).Error; err == nil {
		updates := map[string]any{
			"latest_trends_on": now,
			"modified_on":      now,
		}
		switch action {
		case cs.MetricActionCreateTweet:
			updates["tweets_count"] = gorm.Expr("tweets_count + 1")
		case cs.MetricActionDeleteTweet:
			// 减到0为止 避免并发下减成负数
			updates["tweets_count"] = gorm.Expr("CASE WHEN tweets_count > 0 THEN tweets_count - 1 ELSE 0 END")
		}
		// Model+Updates 由GORM软删除插件自动附加 is_del=0 过滤条件
		return s.db.Model(&dbr.UserMetric{}).Where("user_id = ?", userId).Updates(updates).Error
	}
	// 记录不存在时补建一条(与原实现 Save 的语义一致)
	metric = &dbr.UserMetric{
		Model:          &dbr.Model{},
		UserId:         userId,
		LatestTrendsOn: now,
	}
	if action == cs.MetricActionCreateTweet {
		metric.TweetsCount = 1
	}
	return s.db.Create(metric).Error
}

func (s *userMetricSrvA) AddUserMetric(userId int64) (err error) {
	_, err = (&dbr.UserMetric{UserId: userId}).Create(s.db)
	return
}

func (s *userMetricSrvA) DeleteUserMetric(userId int64) (err error) {
	return (&dbr.UserMetric{UserId: userId}).Delete(s.db)
}

func newTweetMetricServentA(db *gorm.DB) core.TweetMetricServantA {
	return &tweetMetricSrvA{
		db: db,
	}
}

func newCommentMetricServentA(db *gorm.DB) core.CommentMetricServantA {
	return &commentMetricSrvA{
		db: db,
	}
}

func newUserMetricServentA(db *gorm.DB) core.UserMetricServantA {
	return &userMetricSrvA{
		db: db,
	}
}
