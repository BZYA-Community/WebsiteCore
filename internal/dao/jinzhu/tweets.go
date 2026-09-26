// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package jinzhu

import (
	"fmt"
	"strings"
	"time"

	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/core/cs"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/jinzhu/dbr"
	"github.com/BZYA-Community/WebsiteCore/pkg/debug"
	"gorm.io/gorm"
)

type tweetSrv struct {
	db *gorm.DB
}

type tweetManageSrv struct {
	cacheIndex core.CacheIndexService
	db         *gorm.DB
}

type tweetHelpSrv struct {
	db *gorm.DB
}

type tweetSrvA struct {
	db *gorm.DB
}

type tweetManageSrvA struct {
	db *gorm.DB
}

type tweetHelpSrvA struct {
	db *gorm.DB
}

func newTweetService(db *gorm.DB) core.TweetService {
	return &tweetSrv{
		db: db,
	}
}

func newTweetManageService(db *gorm.DB, cacheIndex core.CacheIndexService) core.TweetManageService {
	return &tweetManageSrv{
		cacheIndex: cacheIndex,
		db:         db,
	}
}

func newTweetHelpService(db *gorm.DB) core.TweetHelpService {
	return &tweetHelpSrv{
		db: db,
	}
}

func newTweetServantA(db *gorm.DB) core.TweetServantA {
	return &tweetSrvA{
		db: db,
	}
}

func newTweetManageServantA(db *gorm.DB) core.TweetManageServantA {
	return &tweetManageSrvA{
		db: db,
	}
}

func newTweetHelpServantA(db *gorm.DB) core.TweetHelpServantA {
	return &tweetHelpSrvA{
		db: db,
	}
}

// MergePosts post数据整合
func (s *tweetHelpSrv) MergePosts(posts []*ms.Post) ([]*ms.PostFormated, error) {
	postIds := make([]int64, 0, len(posts))
	userIds := make([]int64, 0, len(posts))
	for _, post := range posts {
		postIds = append(postIds, post.ID)
		userIds = append(userIds, post.UserID)
	}

	postContents, err := s.getPostContentsByIDs(postIds)
	if err != nil {
		return nil, err
	}

	users, err := s.getUsersByIDs(userIds)
	if err != nil {
		return nil, err
	}

	userMap := make(map[int64]*dbr.UserFormated, len(users))
	for _, user := range users {
		userMap[user.ID] = user.Format()
	}

	contentMap := make(map[int64][]*dbr.PostContentFormated, len(postContents))
	for _, content := range postContents {
		contentMap[content.PostID] = append(contentMap[content.PostID], content.Format())
	}

	// 数据整合
	postsFormated := make([]*dbr.PostFormated, 0, len(posts))
	for _, post := range posts {
		postFormated := post.Format()
		postFormated.User = userMap[post.UserID]
		if postFormated.User == nil {
			postFormated.User = ms.GhostUserFormated
		}
		postFormated.Contents = contentMap[post.ID]
		postsFormated = append(postsFormated, postFormated)
	}
	return postsFormated, nil
}

// RevampPosts post数据整形修复
func (s *tweetHelpSrv) RevampPosts(posts []*ms.PostFormated) ([]*ms.PostFormated, error) {
	postIds := make([]int64, 0, len(posts))
	userIds := make([]int64, 0, len(posts))
	for _, post := range posts {
		postIds = append(postIds, post.ID)
		userIds = append(userIds, post.UserID)
	}

	postContents, err := s.getPostContentsByIDs(postIds)
	if err != nil {
		return nil, err
	}

	users, err := s.getUsersByIDs(userIds)
	if err != nil {
		return nil, err
	}

	userMap := make(map[int64]*dbr.UserFormated, len(users))
	for _, user := range users {
		userMap[user.ID] = user.Format()
	}

	contentMap := make(map[int64][]*dbr.PostContentFormated, len(postContents))
	for _, content := range postContents {
		contentMap[content.PostID] = append(contentMap[content.PostID], content.Format())
	}

	// 数据整合
	for _, post := range posts {
		post.User = userMap[post.UserID]
		if post.User == nil {
			post.User = ms.GhostUserFormated
		}
		post.Contents = contentMap[post.ID]
	}
	return posts, nil
}

func (s *tweetHelpSrv) getPostContentsByIDs(ids []int64) ([]*dbr.PostContent, error) {
	return (&dbr.PostContent{}).List(s.db, &dbr.ConditionsT{
		"post_id IN ?": ids,
		"ORDER":        "sort ASC",
	}, 0, 0)
}

func (s *tweetHelpSrv) getUsersByIDs(ids []int64) ([]*dbr.User, error) {
	user := &dbr.User{}

	return user.List(s.db, &dbr.ConditionsT{
		"id IN ?": ids,
	}, 0, 0)
}

func (s *tweetManageSrv) CreatePostCollection(postID, userID int64) (*ms.PostCollection, error) {
	collection := &dbr.PostCollection{
		PostID: postID,
		UserID: userID,
	}

	return collection.Create(s.db)
}

func (s *tweetManageSrv) DeletePostCollection(p *ms.PostCollection) error {
	return p.Delete(s.db)
}

func (s *tweetManageSrv) CreatePostContent(content *ms.PostContent) (*ms.PostContent, error) {
	return content.Create(s.db)
}

func (s *tweetManageSrv) CreateAttachment(obj *ms.Attachment) (int64, error) {
	attachment, err := obj.Create(s.db)
	return attachment.ID, err
}

func (s *tweetManageSrv) CreatePost(post *ms.Post) (*ms.Post, error) {
	post.LatestRepliedOn = time.Now().Unix()
	p, err := post.Create(s.db)
	if err != nil {
		return nil, err
	}
	s.cacheIndex.SendAction(core.IdxActCreatePost, post)
	return p, nil
}

func (s *tweetManageSrv) DeletePost(post *ms.Post) ([]string, error) {
	var mediaContents []string
	postId := post.ID
	postContent := &dbr.PostContent{}
	err := s.db.Transaction(
		func(tx *gorm.DB) error {
			if contents, err := postContent.MediaContentsByPostId(tx, postId); err == nil {
				mediaContents = contents
			} else {
				return err
			}

			// 删推文
			if err := post.Delete(tx); err != nil {
				return err
			}

			// 删内容
			if err := postContent.DeleteByPostId(tx, postId); err != nil {
				return err
			}

			// 删评论
			if contents, err := s.deleteCommentByPostId(tx, postId); err == nil {
				mediaContents = append(mediaContents, contents...)
			} else {
				return err
			}

			if tags := strings.Split(post.Tags, ","); len(tags) > 0 {
				// 删tag，宽松处理错误，有错误不会回滚
				deleteTags(tx, tags)
			}

			return nil
		},
	)
	if err != nil {
		return nil, err
	}

	s.cacheIndex.SendAction(core.IdxActDeletePost, post)
	return mediaContents, nil
}

func (s *tweetManageSrv) deleteCommentByPostId(db *gorm.DB, postId int64) ([]string, error) {
	comment := &dbr.Comment{}
	commentContent := &dbr.CommentContent{}

	// 获取推文的所有评论id
	commentIds, err := comment.CommentIdsByPostId(db, postId)
	if err != nil {
		return nil, err
	}

	// 获取评论的媒体内容
	mediaContents, err := commentContent.MediaContentsByCommentId(db, commentIds)
	if err != nil {
		return nil, err
	}

	// 删评论
	if err = comment.DeleteByPostId(db, postId); err != nil {
		return nil, err
	}

	// 删评论内容
	if err = commentContent.DeleteByCommentIds(db, commentIds); err != nil {
		return nil, err
	}

	// 删评论的评论
	if err = (&dbr.CommentReply{}).DeleteByCommentIds(db, commentIds); err != nil {
		return nil, err
	}

	return mediaContents, nil
}

func (s *tweetManageSrv) LockPost(post *ms.Post) error {
	post.IsLock = 1 - post.IsLock
	return post.Update(s.db)
}

func (s *tweetManageSrv) StickPost(post *ms.Post) error {
	post.IsTop = 1 - post.IsTop
	if err := post.Update(s.db); err != nil {
		return err
	}
	s.cacheIndex.SendAction(core.IdxActStickPost, post)
	return nil
}

func (s *tweetManageSrv) HighlightPost(userId int64, postId int64) (res int, err error) {
	var post dbr.Post
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if xerr := tx.Where("id = ? AND is_del = 0", postId).First(&post).Error; xerr != nil {
			return xerr
		}
		if post.UserID != userId {
			return cs.ErrNoPermission
		}
		post.IsEssence = 1 - post.IsEssence
		if xerr := post.Update(tx); xerr != nil {
			return xerr
		}
		res = post.IsEssence
		return nil
	})
	return
}

func (s *tweetManageSrv) VisiblePost(post *ms.Post, visibility cs.TweetVisibleType) (err error) {
	oldVisibility := post.Visibility
	post.Visibility = ms.PostVisibleT(visibility)
	// TODO: 这个判断是否可以不要呢
	if oldVisibility == ms.PostVisibleT(visibility) {
		return nil
	}
	// 私密推文 特殊处理
	if visibility == cs.TweetVisitPrivate {
		// 强制取消置顶
		// TODO: 置顶推文用户是否有权设置成私密？ 后续完善
		post.IsTop = 0
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if xerr := post.Update(tx); xerr != nil {
			return xerr
		}
		// tag处理
		tags := strings.Split(post.Tags, ",")
		// TODO: 暂时宽松不处理错误，这里或许可以有优化，后续完善
		if oldVisibility == dbr.PostVisitPrivate {
			// 从私密转为非私密才需要重新创建tag
			createTags(tx, post.UserID, tags)
		} else if visibility == cs.TweetVisitPrivate {
			// 从非私密转为私密才需要删除tag
			deleteTags(tx, tags)
		}
		return nil
	})
	if err != nil {
		return
	}
	s.cacheIndex.SendAction(core.IdxActVisiblePost, post)
	return
}

func (s *tweetManageSrv) UpdatePost(post *ms.Post) (err error) {
	if err = post.Update(s.db); err != nil {
		return
	}
	s.cacheIndex.SendAction(core.IdxActUpdatePost, post)
	return
}

// postCounterColumns 帖子计数列白名单 仅允许对计数列做原子自增/自减
var postCounterColumns = map[string]struct{}{
	"comment_count":    {},
	"upvote_count":     {},
	"collection_count": {},
	"share_count":      {},
}

// IncPostCounter 原子自增/自减帖子计数列 使用SQL表达式就地更新
// 避免"读→改→写"在并发下丢计数 同时只写计数列与latest_replied_on 不会覆盖其它列的并发修改
func (s *tweetManageSrv) IncPostCounter(post *ms.Post, column string, delta int, latestRepliedOn int64) error {
	if post == nil || post.Model == nil {
		return fmt.Errorf("jinzhu: IncPostCounter requires a loaded post")
	}
	if _, ok := postCounterColumns[column]; !ok {
		return fmt.Errorf("jinzhu: unsupport post counter column %s", column)
	}
	updates := map[string]any{
		column:        gorm.Expr(column+" + ?", delta),
		"modified_on": time.Now().Unix(),
	}
	if latestRepliedOn > 0 {
		updates["latest_replied_on"] = latestRepliedOn
	}
	// Model+Updates 由GORM软删除插件自动附加 is_del=0 过滤条件
	if err := s.db.Model(&dbr.Post{}).Where("id = ?", post.ID).Updates(updates).Error; err != nil {
		return err
	}
	// 同步内存值 保证后续索引推送与度量更新使用最新计数
	switch column {
	case "comment_count":
		post.CommentCount += int64(delta)
	case "upvote_count":
		post.UpvoteCount += int64(delta)
	case "collection_count":
		post.CollectionCount += int64(delta)
	case "share_count":
		post.ShareCount += int64(delta)
	}
	if latestRepliedOn > 0 {
		post.LatestRepliedOn = latestRepliedOn
	}
	s.cacheIndex.SendAction(core.IdxActUpdatePost, post)
	return nil
}

func (s *tweetManageSrv) CreatePostStar(postID, userID int64) (*ms.PostStar, error) {
	star := &dbr.PostStar{
		PostID: postID,
		UserID: userID,
	}
	return star.Create(s.db)
}

func (s *tweetManageSrv) DeletePostStar(p *ms.PostStar) error {
	return p.Delete(s.db)
}

func (s *tweetSrv) GetPostByID(id int64) (*ms.Post, error) {
	post := &dbr.Post{
		Model: &dbr.Model{
			ID: id,
		},
	}
	return post.Get(s.db)
}

func (s *tweetSrv) GetPosts(conditions ms.ConditionsT, offset, limit int) ([]*ms.Post, error) {
	return (&dbr.Post{}).List(s.db, conditions, offset, limit)
}

func (s *tweetSrv) ListUserTweets(userId int64, style uint8, justEssence bool, limit, offset int) (res []*ms.Post, total int64, err error) {
	db := s.db.Model(&dbr.Post{}).Where("user_id = ?", userId)
	switch style {
	case cs.StyleUserTweetsAdmin:
		fallthrough
	case cs.StyleUserTweetsSelf:
		// 作者/管理员可见全部状态(含待审核)
		db = db.Where("visibility >= ?", cs.TweetVisitPrivate)
	case cs.StyleUserTweetsFriend:
		db = db.Where("visibility >= ? AND audit_status = ?", cs.TweetVisitFriend, dbr.PostAuditApproved)
	case cs.StyleUserTweetsFollowing:
		db = db.Where("visibility >= ? AND audit_status = ?", cs.TweetVisitFollowing, dbr.PostAuditApproved)
	case cs.StyleUserTweetsGuest:
		fallthrough
	default:
		db = db.Where("visibility >= ? AND audit_status = ?", cs.TweetVisitPublic, dbr.PostAuditApproved)
	}
	if justEssence {
		db = db.Where("is_essence=1")
	}
	if err = db.Count(&total).Error; err != nil {
		return
	}
	if offset >= 0 && limit > 0 {
		db = db.Offset(offset).Limit(limit)
	}
	if err = db.Order("is_top DESC, latest_replied_on DESC").Find(&res).Error; err != nil {
		return
	}
	return
}

func (s *tweetSrv) ListIndexNewestTweets(limit, offset int) (res []*ms.Post, total int64, err error) {
	// 公共广场仅展示已过审的公开帖
	db := s.db.Table(_post_).Where("visibility >= ? AND audit_status = ?", cs.TweetVisitPublic, dbr.PostAuditApproved)
	if err = db.Count(&total).Error; err != nil {
		return
	}
	if offset >= 0 && limit > 0 {
		db = db.Offset(offset).Limit(limit)
	}
	if err = db.Order("is_top DESC, latest_replied_on DESC").Find(&res).Error; err != nil {
		return
	}
	return
}

func (s *tweetSrv) ListIndexHotsTweets(limit, offset int) (res []*ms.Post, total int64, err error) {
	db := s.db.Table(_post_).Joins(fmt.Sprintf("LEFT JOIN %s metric ON %s.id=metric.post_id", _post_metric_, _post_)).Where(fmt.Sprintf("visibility >= ? AND audit_status = ? AND %s.is_del=0 AND metric.is_del=0", _post_), cs.TweetVisitPublic, dbr.PostAuditApproved)
	if err = db.Count(&total).Error; err != nil {
		return
	}
	if offset >= 0 && limit > 0 {
		db = db.Offset(offset).Limit(limit)
	}
	if err = db.Order("is_top DESC, metric.rank_score DESC, latest_replied_on DESC").Find(&res).Error; err != nil {
		return
	}
	return
}

func (s *tweetSrv) ListSyncSearchTweets(limit, offset int) (res []*ms.Post, total int64, err error) {
	// 搜索引擎仅同步已过审的帖子
	db := s.db.Table(_post_).Where("visibility >= ? AND audit_status = ?", cs.TweetVisitFriend, dbr.PostAuditApproved)
	if err = db.Count(&total).Error; err != nil {
		return
	}
	if offset >= 0 && limit > 0 {
		db = db.Offset(offset).Limit(limit)
	}
	if err = db.Find(&res).Error; err != nil {
		return
	}
	return
}

func (s *tweetSrv) ListFollowingTweets(userId int64, limit, offset int) (res []*ms.Post, total int64, err error) {
	// 好友功能已移除: 仅按关注关系查询(关注可见=60 公开=90 均满足 visibility>=60)
	var beFollowIds []int64
	if err = s.db.Table(_following_).Where("user_id=? AND is_del=0", userId).Select("follow_id").Find(&beFollowIds).Error; err != nil {
		return
	}
	db := s.db.Model(&dbr.Post{})
	// 私密帖免审仅作者可见 关注/公开帖需过审后才对他人可见
	if len(beFollowIds) > 0 {
		db = db.Where("user_id=? OR ((visibility>=60 AND audit_status=1) AND user_id IN(?))", userId, beFollowIds)
	} else {
		db = db.Where("user_id = ?", userId)
	}
	if err = db.Count(&total).Error; err != nil {
		return
	}
	if offset >= 0 && limit > 0 {
		db = db.Offset(offset).Limit(limit)
	}
	if err = db.Order("is_top DESC, latest_replied_on DESC").Find(&res).Error; err != nil {
		return
	}
	return
}

func (s *tweetSrv) GetPostCount(conditions ms.ConditionsT) (int64, error) {
	return (&dbr.Post{}).Count(s.db, conditions)
}

func (s *tweetSrv) GetUserPostStar(postID, userID int64) (*ms.PostStar, error) {
	star := &dbr.PostStar{
		PostID: postID,
		UserID: userID,
	}
	return star.Get(s.db)
}

func (s *tweetSrv) GetUserPostStars(userID int64, limit int, offset int) ([]*ms.PostStar, error) {
	star := &dbr.PostStar{
		UserID: userID,
	}
	return star.List(s.db, &dbr.ConditionsT{
		"ORDER": s.db.NamingStrategy.TableName("PostStar") + ".id DESC",
	}, cs.RelationSelf, limit, offset)
}

func (s *tweetSrv) ListUserStarTweets(user *cs.VistUser, limit int, offset int) (res []*ms.PostStar, total int64, err error) {
	star := &dbr.PostStar{
		UserID: user.UserId,
	}
	if total, err = star.Count(s.db, user.RelTyp, &dbr.ConditionsT{}); err != nil {
		return
	}
	res, err = star.List(s.db, &dbr.ConditionsT{
		"ORDER": s.db.NamingStrategy.TableName("PostStar") + ".id DESC",
	}, user.RelTyp, limit, offset)
	return
}

func (s *tweetSrv) getUserTweets(db *gorm.DB, user *cs.VistUser, limit int, offset int) (res []*ms.Post, total int64, err error) {
	visibilities := []core.PostVisibleT{core.PostVisitPublic}
	switch user.RelTyp {
	// 好友功能已移除: 好友可见按私密口径(仅作者/管理员可见)
	case cs.RelationAdmin, cs.RelationSelf:
		visibilities = append(visibilities, core.PostVisitPrivate, core.PostVisitFriend)
	case cs.RelationGuest:
		fallthrough
	default:
		// nothing
	}
	db = db.Where("visibility IN ? AND is_del=0", visibilities)
	// 非作者/管理员关系: 仅已过审帖可见(私密帖本就不在可见性列表中)
	if user.RelTyp != cs.RelationAdmin && user.RelTyp != cs.RelationSelf {
		db = db.Where("audit_status = ?", dbr.PostAuditApproved)
	}
	err = db.Count(&total).Error
	if err != nil {
		return
	}
	if offset >= 0 && limit > 0 {
		db = db.Offset(offset).Limit(limit)
	}
	err = db.Order("latest_replied_on DESC").Find(&res).Error
	return
}

func (s *tweetSrv) ListUserMediaTweets(user *cs.VistUser, limit int, offset int) ([]*ms.Post, int64, error) {
	db := s.db.Table(_post_by_media_).Where("user_id=?", user.UserId)
	return s.getUserTweets(db, user, limit, offset)
}

func (s *tweetSrv) ListUserCommentTweets(user *cs.VistUser, limit int, offset int) ([]*ms.Post, int64, error) {
	db := s.db.Table(_post_by_comment_).Where("comment_user_id=?", user.UserId)
	return s.getUserTweets(db, user, limit, offset)
}

func (s *tweetSrv) GetUserPostStarCount(userID int64) (int64, error) {
	star := &dbr.PostStar{
		UserID: userID,
	}
	return star.Count(s.db, cs.RelationSelf, &dbr.ConditionsT{})
}

func (s *tweetSrv) GetUserPostCollection(postID, userID int64) (*ms.PostCollection, error) {
	star := &dbr.PostCollection{
		PostID: postID,
		UserID: userID,
	}
	return star.Get(s.db)
}

func (s *tweetSrv) GetUserPostCollections(userID int64, offset, limit int) ([]*ms.PostCollection, error) {
	collection := &dbr.PostCollection{
		UserID: userID,
	}

	return collection.List(s.db, &dbr.ConditionsT{
		"ORDER": s.db.NamingStrategy.TableName("PostCollection") + ".id DESC",
	}, offset, limit)
}

func (s *tweetSrv) GetUserPostCollectionCount(userID int64) (int64, error) {
	collection := &dbr.PostCollection{
		UserID: userID,
	}
	return collection.Count(s.db, &dbr.ConditionsT{})
}

func (s *tweetSrv) GetPostContentsByIDs(ids []int64) ([]*ms.PostContent, error) {
	return (&dbr.PostContent{}).List(s.db, &dbr.ConditionsT{
		"post_id IN ?": ids,
		"ORDER":        "sort ASC",
	}, 0, 0)
}

func (s *tweetSrv) GetPostContentByID(id int64) (*ms.PostContent, error) {
	return (&dbr.PostContent{
		Model: &dbr.Model{
			ID: id,
		},
	}).Get(s.db)
}

func (s *tweetSrvA) TweetInfoById(id int64) (*cs.TweetInfo, error) {
	// TODO
	return nil, debug.ErrNotImplemented
}

func (s *tweetSrvA) TweetItemById(id int64) (*cs.TweetItem, error) {
	// TODO
	return nil, debug.ErrNotImplemented
}

func (s *tweetSrvA) UserTweets(visitorId, userId int64) (cs.TweetList, error) {
	// TODO
	return nil, debug.ErrNotImplemented
}

func (s *tweetSrvA) ReactionByTweetId(userId int64, tweetId int64) (*cs.ReactionItem, error) {
	// TODO
	return nil, debug.ErrNotImplemented
}

func (s *tweetSrvA) UserReactions(userId int64, offset int, limit int) (cs.ReactionList, error) {
	// TODO
	return nil, debug.ErrNotImplemented
}

func (s *tweetSrvA) FavoriteByTweetId(userId int64, tweetId int64) (*cs.FavoriteItem, error) {
	// TODO
	return nil, debug.ErrNotImplemented
}

func (s *tweetSrvA) UserFavorites(userId int64, offset int, limit int) (cs.FavoriteList, error) {
	// TODO
	return nil, debug.ErrNotImplemented
}

func (s *tweetSrvA) AttachmentByTweetId(userId int64, tweetId int64) (*cs.AttachmentBill, error) {
	// TODO
	return nil, debug.ErrNotImplemented
}

func (s *tweetManageSrvA) CreateAttachment(obj *cs.Attachment) (int64, error) {
	// TODO
	return 0, debug.ErrNotImplemented
}

func (s *tweetManageSrvA) CreateTweet(userId int64, req *cs.NewTweetReq) (*cs.TweetItem, error) {
	// TODO
	return nil, debug.ErrNotImplemented
}

func (s *tweetManageSrvA) DeleteTweet(userId int64, tweetId int64) ([]string, error) {
	// TODO
	return nil, debug.ErrNotImplemented
}

func (s *tweetManageSrvA) LockTweet(userId int64, tweetId int64) error {
	// TODO
	return debug.ErrNotImplemented
}

func (s *tweetManageSrvA) StickTweet(userId int64, tweetId int64) error {
	// TODO
	return debug.ErrNotImplemented
}

func (s *tweetManageSrvA) VisibleTweet(userId int64, visibility cs.TweetVisibleType) error {
	// TODO
	return debug.ErrNotImplemented
}

func (s *tweetManageSrvA) CreateReaction(userId int64, tweetId int64) error {
	// TODO
	return debug.ErrNotImplemented
}

func (s *tweetManageSrvA) DeleteReaction(userId int64, reactionId int64) error {
	// TODO
	return debug.ErrNotImplemented
}

func (s *tweetManageSrvA) CreateFavorite(userId int64, tweetId int64) error {
	// TODO
	return debug.ErrNotImplemented
}

func (s *tweetManageSrvA) DeleteFavorite(userId int64, favoriteId int64) error {
	// TODO
	return debug.ErrNotImplemented
}

func (s *tweetHelpSrvA) RevampTweets(tweets cs.TweetList) (cs.TweetList, error) {
	// TODO
	return nil, debug.ErrNotImplemented
}

func (s *tweetHelpSrvA) MergeTweets(tweets cs.TweetInfo) (cs.TweetList, error) {
	// TODO
	return nil, debug.ErrNotImplemented
}
