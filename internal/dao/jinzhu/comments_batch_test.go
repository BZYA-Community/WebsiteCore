// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package jinzhu

import (
	"fmt"
	"testing"

	"github.com/BZYA-Community/WebsiteCore/internal/dao/jinzhu/dbr"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
	_ "modernc.org/sqlite"
)

// newCommentsTestDB 评论/回复相关测试用内存库(无需config.yaml)
func newCommentsTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(&sqlite.Dialector{DriverName: "sqlite", DSN: dsn}, &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "p_", SingularTable: true},
	})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	if err := db.AutoMigrate(&dbr.Comment{}, &dbr.CommentReply{}, &dbr.CourseCommentReply{}, &dbr.TweetCommentThumbs{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	return db
}

// TestGetCommentRepliesByReplyIDs 批量按回复id取回复: 一次查询 软删除与缺失id不返回 空入参不查库
func TestGetCommentRepliesByReplyIDs(t *testing.T) {
	db := newCommentsTestDB(t)
	srv := newCommentService(db)

	ids := make([]int64, 0, 3)
	for i := 1; i <= 3; i++ {
		reply := &dbr.CommentReply{
			Model:     &dbr.Model{},
			CommentID: 10,
			UserID:    2,
			Content:   fmt.Sprintf("reply-%d", i),
		}
		if err := db.Create(reply).Error; err != nil {
			t.Fatalf("Create reply error = %v", err)
		}
		ids = append(ids, reply.ID)
	}
	// 软删除第2条
	if err := (&dbr.CommentReply{Model: &dbr.Model{ID: ids[1]}}).Delete(db); err != nil {
		t.Fatalf("Delete reply error = %v", err)
	}

	got, err := srv.GetCommentRepliesByReplyIDs([]int64{ids[0], ids[1], ids[2], 99999})
	if err != nil {
		t.Fatalf("GetCommentRepliesByReplyIDs() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2 (软删除与缺失id不返回)", len(got))
	}
	contents := make(map[int64]string, len(got))
	for _, r := range got {
		contents[r.ID] = r.Content
	}
	if contents[ids[0]] != "reply-1" || contents[ids[2]] != "reply-3" {
		t.Fatalf("contents = %v, want reply-1与reply-3", contents)
	}

	// 空入参不查库也不报错
	if got, err := srv.GetCommentRepliesByReplyIDs(nil); err != nil || len(got) != 0 {
		t.Fatalf("GetCommentRepliesByReplyIDs(nil) = (%v, %v), want (空, nil)", got, err)
	}
}

// TestGetCourseCommentRepliesByReplyIDs 课程回复批量取回 同上口径
func TestGetCourseCommentRepliesByReplyIDs(t *testing.T) {
	db := newCommentsTestDB(t)
	srv := newCourseService(db)

	ids := make([]int64, 0, 2)
	for i := 1; i <= 2; i++ {
		reply := &dbr.CourseCommentReply{
			Model:     &dbr.Model{},
			CommentID: 20,
			UserID:    2,
			Content:   fmt.Sprintf("course-reply-%d", i),
		}
		if err := db.Create(reply).Error; err != nil {
			t.Fatalf("Create course reply error = %v", err)
		}
		ids = append(ids, reply.ID)
	}
	if err := (&dbr.CourseCommentReply{Model: &dbr.Model{ID: ids[0]}}).Delete(db); err != nil {
		t.Fatalf("Delete course reply error = %v", err)
	}

	got, err := srv.GetCourseCommentRepliesByReplyIDs([]int64{ids[0], ids[1], 99999})
	if err != nil {
		t.Fatalf("GetCourseCommentRepliesByReplyIDs() error = %v", err)
	}
	if len(got) != 1 || got[0].ID != ids[1] || got[0].Content != "course-reply-2" {
		t.Fatalf("got = %+v, want 仅course-reply-2", got)
	}
	if got, err := srv.GetCourseCommentRepliesByReplyIDs(nil); err != nil || len(got) != 0 {
		t.Fatalf("GetCourseCommentRepliesByReplyIDs(nil) = (%v, %v), want (空, nil)", got, err)
	}
}

// TestThumbsShareOneToggleLogic ThumbsUp/Down × Comment/Reply 四个入口共用一套状态机:
// 评论与回复的赞态/计数互相独立 且各状态切换的计数增减与收敛前一致
func TestThumbsShareOneToggleLogic(t *testing.T) {
	db := newCommentsTestDB(t)
	srv := newCommentManageService(db)

	comment := &dbr.Comment{Model: &dbr.Model{}, PostID: 1, UserID: 2}
	if err := db.Create(comment).Error; err != nil {
		t.Fatalf("Create comment error = %v", err)
	}
	reply := &dbr.CommentReply{Model: &dbr.Model{}, CommentID: comment.ID, UserID: 3, Content: "reply"}
	if err := db.Create(reply).Error; err != nil {
		t.Fatalf("Create reply error = %v", err)
	}

	const (
		userId  = 7
		tweetId = 1
	)

	reloadComment := func() *dbr.Comment {
		t.Helper()
		var c dbr.Comment
		if err := db.Where("id = ?", comment.ID).First(&c).Error; err != nil {
			t.Fatalf("reload comment error = %v", err)
		}
		return &c
	}
	reloadReply := func() *dbr.CommentReply {
		t.Helper()
		var r dbr.CommentReply
		if err := db.Where("id = ?", reply.ID).First(&r).Error; err != nil {
			t.Fatalf("reload reply error = %v", err)
		}
		return &r
	}
	// 赞态按(用户,帖子,评论,comment_type)取 评论与回复各自一行
	takeThumbs := func(commentType int8, replyId int64) *dbr.TweetCommentThumbs {
		t.Helper()
		var th dbr.TweetCommentThumbs
		if err := db.Where("user_id = ? AND tweet_id = ? AND comment_id = ? AND comment_type = ?",
			userId, tweetId, comment.ID, commentType).First(&th).Error; err != nil {
			t.Fatalf("take thumbs[comment_type=%d] error = %v", commentType, err)
		}
		if th.ReplyID != replyId {
			t.Fatalf("ReplyID = %d, want %d", th.ReplyID, replyId)
		}
		return &th
	}
	assertCommentCounts := func(up, down int32) {
		t.Helper()
		if c := reloadComment(); c.ThumbsUpCount != up || c.ThumbsDownCount != down {
			t.Fatalf("comment counts = (%d,%d), want (%d,%d)", c.ThumbsUpCount, c.ThumbsDownCount, up, down)
		}
	}
	assertReplyCounts := func(up, down int32) {
		t.Helper()
		if r := reloadReply(); r.ThumbsUpCount != up || r.ThumbsDownCount != down {
			t.Fatalf("reply counts = (%d,%d), want (%d,%d)", r.ThumbsUpCount, r.ThumbsDownCount, up, down)
		}
	}
	assertFlags := func(commentType int8, replyId int64, up, down int8) {
		t.Helper()
		th := takeThumbs(commentType, replyId)
		if th.IsThumbsUp != up || th.IsThumbsDown != down {
			t.Fatalf("flags[comment_type=%d] = (%d,%d), want (%d,%d)", commentType, th.IsThumbsUp, th.IsThumbsDown, up, down)
		}
	}

	// 评论: 首赞(新建赞态) -> 取消赞 -> 点踩 -> 赞踩并存时点赞 -> 已赞时点踩 -> 取消踩
	steps := []struct {
		name       string
		up         bool
		wantCounts [2]int32
		wantFlags  [2]int8
	}{
		{"首赞", true, [2]int32{1, 0}, [2]int8{1, 0}},
		{"取消赞", true, [2]int32{0, 0}, [2]int8{0, 0}},
		{"点踩", false, [2]int32{0, 1}, [2]int8{0, 1}},
		{"赞踩并存时点赞", true, [2]int32{1, 0}, [2]int8{1, 0}},
		{"已赞时点踩", false, [2]int32{0, 1}, [2]int8{0, 1}},
		{"取消踩", false, [2]int32{0, 0}, [2]int8{0, 0}},
	}
	for _, step := range steps {
		var err error
		if step.up {
			err = srv.ThumbsUpComment(userId, tweetId, comment.ID)
		} else {
			err = srv.ThumbsDownComment(userId, tweetId, comment.ID)
		}
		if err != nil {
			t.Fatalf("%s error = %v", step.name, err)
		}
		assertCommentCounts(step.wantCounts[0], step.wantCounts[1])
		assertFlags(0, 0, step.wantFlags[0], step.wantFlags[1])
	}

	// 回复: 同一状态机 但落独立的赞态行(comment_type=1)与回复表计数
	for _, step := range steps {
		var err error
		if step.up {
			err = srv.ThumbsUpReply(userId, tweetId, comment.ID, reply.ID)
		} else {
			err = srv.ThumbsDownReply(userId, tweetId, comment.ID, reply.ID)
		}
		if err != nil {
			t.Fatalf("reply %s error = %v", step.name, err)
		}
		assertReplyCounts(step.wantCounts[0], step.wantCounts[1])
		assertFlags(1, reply.ID, step.wantFlags[0], step.wantFlags[1])
		// 回复点赞不影响评论计数与评论赞态行(最终评论侧停留在(0,0))
		assertCommentCounts(0, 0)
		assertFlags(0, 0, 0, 0)
	}

	// 仅两行赞态(评论一行/回复一行) 无重复插入
	var n int64
	if err := db.Model(&dbr.TweetCommentThumbs{}).Where("user_id = ? AND tweet_id = ?", userId, tweetId).Count(&n).Error; err != nil {
		t.Fatalf("count thumbs error = %v", err)
	}
	if n != 2 {
		t.Fatalf("thumbs rows = %d, want 2", n)
	}
}

// TestThumbsUpCorruptStateKeepsLegacySemantics 历史差异: 赞态同时为"赞+踩"的异常数据下
// 评论上赞走默认分支(+1赞/-1踩并清踩) 回复上赞按"取消赞"计(-1赞) 收敛后语义保持不变
func TestThumbsUpCorruptStateKeepsLegacySemantics(t *testing.T) {
	db := newCommentsTestDB(t)
	srv := newCommentManageService(db)

	comment := &dbr.Comment{Model: &dbr.Model{}, PostID: 1, UserID: 2}
	if err := db.Create(comment).Error; err != nil {
		t.Fatalf("Create comment error = %v", err)
	}
	reply := &dbr.CommentReply{Model: &dbr.Model{}, CommentID: comment.ID, UserID: 3, Content: "reply"}
	if err := db.Create(reply).Error; err != nil {
		t.Fatalf("Create reply error = %v", err)
	}
	const (
		userId  = 7
		tweetId = 1
	)
	corrupt := func(commentType int8, replyId int64) {
		t.Helper()
		if err := db.Create(&dbr.TweetCommentThumbs{
			Model:        &dbr.Model{},
			UserID:       userId,
			TweetID:      tweetId,
			CommentID:    comment.ID,
			ReplyID:      replyId,
			CommentType:  commentType,
			IsThumbsUp:   1,
			IsThumbsDown: 1,
		}).Error; err != nil {
			t.Fatalf("create corrupt thumbs error = %v", err)
		}
	}
	setCounts := func(model any, id int64, up, down int32) {
		t.Helper()
		if err := db.Model(model).Where("id = ?", id).Updates(map[string]any{
			"thumbs_up_count":   up,
			"thumbs_down_count": down,
		}).Error; err != nil {
			t.Fatalf("set counts error = %v", err)
		}
	}
	corrupt(0, 0)
	corrupt(1, reply.ID)
	setCounts(&dbr.Comment{}, comment.ID, 5, 5)
	setCounts(&dbr.CommentReply{}, reply.ID, 5, 5)

	// 评论: 异常态上赞 -> 默认分支 +1赞/-1踩 且清掉踩标记
	if err := srv.ThumbsUpComment(userId, tweetId, comment.ID); err != nil {
		t.Fatalf("ThumbsUpComment error = %v", err)
	}
	var c dbr.Comment
	if err := db.Where("id = ?", comment.ID).First(&c).Error; err != nil {
		t.Fatalf("reload comment error = %v", err)
	}
	if c.ThumbsUpCount != 6 || c.ThumbsDownCount != 4 {
		t.Fatalf("comment counts = (%d,%d), want (6,4)", c.ThumbsUpCount, c.ThumbsDownCount)
	}
	var th dbr.TweetCommentThumbs
	if err := db.Where("user_id = ? AND comment_type = 0", userId).First(&th).Error; err != nil {
		t.Fatalf("take comment thumbs error = %v", err)
	}
	if th.IsThumbsUp != 0 || th.IsThumbsDown != 0 {
		t.Fatalf("comment flags = (%d,%d), want (0,0)", th.IsThumbsUp, th.IsThumbsDown)
	}

	// 回复: 异常态上赞 -> 取消赞分支 -1赞 不动踩计数
	if err := srv.ThumbsUpReply(userId, tweetId, comment.ID, reply.ID); err != nil {
		t.Fatalf("ThumbsUpReply error = %v", err)
	}
	var r dbr.CommentReply
	if err := db.Where("id = ?", reply.ID).First(&r).Error; err != nil {
		t.Fatalf("reload reply error = %v", err)
	}
	if r.ThumbsUpCount != 4 || r.ThumbsDownCount != 5 {
		t.Fatalf("reply counts = (%d,%d), want (4,5)", r.ThumbsUpCount, r.ThumbsDownCount)
	}
	// 重新声明避免上一次First残留的主键被带入查询条件
	var thReply dbr.TweetCommentThumbs
	if err := db.Where("user_id = ? AND comment_type = 1", userId).First(&thReply).Error; err != nil {
		t.Fatalf("take reply thumbs error = %v", err)
	}
	if thReply.IsThumbsUp != 0 || thReply.IsThumbsDown != 1 {
		t.Fatalf("reply flags = (%d,%d), want (0,1)", thReply.IsThumbsUp, thReply.IsThumbsDown)
	}
}
