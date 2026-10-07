// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package web

import (
	"context"
	"fmt"
	"unicode/utf8"

	api "github.com/BZYA-Community/WebsiteCore/auto/api/v1"
	"github.com/BZYA-Community/WebsiteCore/internal/conf"
	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/jinzhu/dbr"
	"github.com/BZYA-Community/WebsiteCore/internal/model/joint"
	"github.com/BZYA-Community/WebsiteCore/internal/model/web"
	"github.com/BZYA-Community/WebsiteCore/internal/servants/base"
	"github.com/BZYA-Community/WebsiteCore/internal/servants/chain"
	"github.com/BZYA-Community/WebsiteCore/pkg/xerror"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type coreSrv struct {
	api.UnimplementedCoreServant
	*base.DaoServant
	oss            core.ObjectStorageService
	wc             core.WebCache
	messagesExpire int64
	prefixMessages string
}

func (s *coreSrv) Chain() gin.HandlersChain {
	return gin.HandlersChain{chain.JWT(), chain.Authorize()}
}

func (s *coreSrv) SyncSearchIndex(req *web.SyncSearchIndexReq) error {
	if !req.User.HasPermission("site.manage") {
		return web.ErrNoPermission
	}
	s.PushAllPostToSearch()
	return nil
}

func (s *coreSrv) GetUserInfo(req *web.UserInfoReq) (*web.UserInfoResp, error) {
	user, err := s.Ds.UserProfileByName(req.Username)
	if err != nil {
		logrus.Errorf("coreSrv.GetUserInfo occurs error[1]: %s", err)
		return nil, xerror.UnauthorizedAuthNotExist
	}
	follows, followings, err := s.Ds.GetFollowCount(user.ID)
	if err != nil {
		return nil, web.ErrGetFollowCountFailed
	}
	resp := &web.UserInfoResp{
		Email:           maskEmail(req.User.Email),
		ContactVerified: req.User.ContactVerified(),
		Id:              user.ID,
		Nickname:        user.Nickname,
		Username:        user.Username,
		Status:          user.Status,
		Avatar:          user.Avatar,
		IsAdmin:         req.User.HasPermission("user.manage"),
		IsOperator:      req.User.IsOperator,
		IdentityGroups:  req.User.GroupList(),
		Permissions:     req.User.PermissionList(),
		Roles:           req.User.RoleList(),
		Identity:        req.User.DisplayIdentity(),
		CreatedOn:       user.CreatedOn,
		Follows:         follows,
		Followings:      followings,
		TweetsCount:     user.TweetsCount,
	}
	if req.User.Phone != "" {
		resp.Phone = dbr.MaskPhone(req.User.Phone)
	}
	return resp, nil
}

func (s *coreSrv) GetMessages(req *web.GetMessagesReq) (res *web.GetMessagesResp, _ error) {
	limit, offset := req.PageSize, (req.Page-1)*req.PageSize
	// 尝试直接从缓存中获取数据
	key, ok := "", false
	if res, key, ok = s.messagesFromCache(req, limit, offset); ok {
		// logrus.Debugf("coreSrv.GetMessages from cache key:%s", key)
		return
	}
	messages, totalRows, err := s.Ds.GetMessages(req.Uid, req.Style, limit, offset)
	if err != nil {
		logrus.Errorf("Ds.GetMessages err[1]: %s", err)
		return nil, web.ErrGetMessagesFailed
	}
	for _, mf := range messages {
		// TODO: 优化处理这里的user获取逻辑以及错误处理
		if mf.SenderUserID > 0 {
			if user, err := s.Ds.GetUserByID(mf.SenderUserID); err == nil {
				mf.SenderUser = user.Format()
			}
		}
		if mf.Type == ms.MsgTypeWhisper && mf.ReceiverUserID != req.Uid {
			if user, err := s.Ds.GetUserByID(mf.ReceiverUserID); err == nil {
				mf.ReceiverUser = user.Format()
			}
		}
		if mf.PostID > 0 {
			post, err := s.GetTweetBy(mf.PostID)
			if err == nil {
				mf.Post = post
				if mf.CommentID > 0 {
					comment, err := s.Ds.GetCommentByID(mf.CommentID)
					if err == nil {
						mf.Comment = comment
						if mf.ReplyID > 0 {
							reply, err := s.Ds.GetCommentReplyByID(mf.ReplyID)
							if err == nil {
								mf.Reply = reply
							}
						}
					}
				}
			}
		}
	}
	if err != nil {
		logrus.Errorf("Ds.GetMessages err[2]: %s", err)
		return nil, web.ErrGetMessagesFailed
	}
	if err = s.PrepareMessages(req.Uid, messages); err != nil {
		logrus.Errorf("get messages err[3]: %s", err)
		return nil, web.ErrGetMessagesFailed
	}
	resp := joint.PageRespFrom(messages, req.Page, req.PageSize, totalRows)
	// 缓存处理
	base.OnCacheRespEvent(s.wc, key, resp, s.messagesExpire)
	return &web.GetMessagesResp{
		CachePageResp: joint.CachePageResp{
			Data: resp,
		},
	}, nil
}

func (s *coreSrv) ReadMessage(req *web.ReadMessageReq) error {
	message, err := s.Ds.GetMessageByID(req.ID)
	if err != nil {
		return web.ErrReadMessageFailed
	}
	if message.ReceiverUserID != req.Uid {
		return web.ErrNoPermission
	}
	if err = s.Ds.ReadMessage(message); err != nil {
		logrus.Errorf("Ds.ReadMessage err: %s", err)
		return web.ErrReadMessageFailed
	}
	// 缓存处理
	onMessageActionEvent(_messageActionRead, req.Uid)
	return nil
}

func (s *coreSrv) ReadAllMessage(req *web.ReadAllMessageReq) error {
	if err := s.Ds.ReadAllMessage(req.Uid); err != nil {
		logrus.Errorf("coreSrv.Ds.ReadAllMessage err: %s", err)
		return web.ErrReadMessageFailed
	}
	// 缓存处理
	onMessageActionEvent(_messageActionRead, req.Uid)
	return nil
}

func (s *coreSrv) GetCollections(req *web.GetCollectionsReq) (*web.GetCollectionsResp, error) {
	collections, err := s.Ds.GetUserPostCollections(req.UserId, (req.Page-1)*req.PageSize, req.PageSize)
	if err != nil {
		logrus.Errorf("Ds.GetUserPostCollections err: %s", err)
		return nil, web.ErrGetCollectionsFailed
	}
	totalRows, err := s.Ds.GetUserPostCollectionCount(req.UserId)
	if err != nil {
		logrus.Errorf("Ds.GetUserPostCollectionCount err: %s", err)
		return nil, web.ErrGetCollectionsFailed
	}
	var posts []*ms.Post
	for _, collection := range collections {
		posts = append(posts, collection.Post)
	}
	postsFormated, err := s.Ds.MergePosts(posts)
	if err != nil {
		logrus.Errorf("Ds.MergePosts err: %s", err)
		return nil, web.ErrGetCollectionsFailed
	}
	if err = s.PrepareTweets(req.UserId, postsFormated); err != nil {
		logrus.Errorf("get collections prepare tweets err: %s", err)
		return nil, web.ErrGetCollectionsFailed
	}
	resp := joint.PageRespFrom(postsFormated, req.Page, req.PageSize, totalRows)
	return (*web.GetCollectionsResp)(resp), nil
}

func (s *coreSrv) UserPhoneBind(req *web.UserPhoneBindReq) error {
	return contactError(s.Ds.VerifyContactCode(context.Background(), req.User.ID, "phone", req.Phone, req.Captcha))
}

func (s *coreSrv) GetStars(req *web.GetStarsReq) (*web.GetStarsResp, error) {
	stars, err := s.Ds.GetUserPostStars(req.UserId, req.PageSize, (req.Page-1)*req.PageSize)
	if err != nil {
		logrus.Errorf("Ds.GetUserPostStars err: %s", err)
		return nil, web.ErrGetStarsFailed
	}
	totalRows, err := s.Ds.GetUserPostStarCount(req.UserId)
	if err != nil {
		logrus.Errorf("Ds.GetUserPostStars err: %s", err)
		return nil, web.ErrGetStarsFailed
	}
	var posts []*ms.Post
	for _, star := range stars {
		posts = append(posts, star.Post)
	}
	postsFormated, err := s.Ds.MergePosts(posts)
	if err != nil {
		logrus.Errorf("Ds.MergePosts err: %s", err)
		return nil, web.ErrGetStarsFailed
	}
	resp := joint.PageRespFrom(postsFormated, req.Page, req.PageSize, totalRows)
	return (*web.GetStarsResp)(resp), nil
}

func (s *coreSrv) ChangePassword(req *web.ChangePasswordReq) error {
	// 密码检查
	if err := checkPassword(req.Password); err != nil {
		return err
	}
	// 旧密码校验
	user := req.User
	if !validPassword(user.Password, req.OldPassword) {
		return web.ErrErrorOldPassword
	}
	// 更新入库
	user.Password, user.Salt = encryptPasswordAndSalt(req.Password)
	if err := s.Ds.UpdateUser(user, "password", "salt"); err != nil {
		logrus.Errorf("Ds.UpdateUser err: %s", err)
		return xerror.ServerError
	}
	return nil
}

func (s *coreSrv) SuggestTags(req *web.SuggestTagsReq) (*web.SuggestTagsResp, error) {
	tags, err := s.Ds.TagsByKeyword(req.Keyword)
	if err != nil {
		logrus.Errorf("Ds.GetTagsByKeyword err: %s", err)
		return nil, xerror.ServerError
	}
	resp := &web.SuggestTagsResp{}
	for _, t := range tags {
		resp.Suggests = append(resp.Suggests, t.Tag)
	}
	return resp, nil
}

func (s *coreSrv) SuggestUsers(req *web.SuggestUsersReq) (*web.SuggestUsersResp, error) {
	users, err := s.Ds.GetUsersByKeyword(req.Keyword)
	if err != nil {
		logrus.Errorf("Ds.GetUsersByKeyword err: %s", err)
		return nil, xerror.ServerError
	}
	resp := &web.SuggestUsersResp{}
	for _, user := range users {
		resp.Suggests = append(resp.Suggests, user.Username)
	}
	return resp, nil
}

func (s *coreSrv) ChangeNickname(req *web.ChangeNicknameReq) error {
	if utf8.RuneCountInString(req.Nickname) < 2 || utf8.RuneCountInString(req.Nickname) > 12 {
		return web.ErrNicknameLengthLimit
	}
	user := req.User
	if _, err := s.Ds.SubmitProfileReview(user, ms.ReviewNickname, req.Nickname); err != nil {
		return reviewError(err)
	}
	onChangeUsernameEvent(user.ID, user.Username)
	return nil
}

func (s *coreSrv) ChangeAvatar(req *web.ChangeAvatarReq) (*web.ChangeAvatarResp, error) {
	if err := s.Ds.CheckAttachment(req.Avatar); err != nil {
		logrus.Errorf("Ds.CheckAttachment failed: %s", err)
		return nil, xerror.InvalidParams
	}

	// 待审对象也立即持久化: 防止临时对象过期清理在审核完成前误删头像文件
	if err := s.oss.PersistObject(s.oss.ObjectKey(req.Avatar)); err != nil {
		logrus.Errorf("Ds.ChangeUserAvatar persist object failed: %s", err)
		return nil, xerror.ServerError
	}
	user := req.User
	result, err := s.Ds.SubmitProfileReview(user, ms.ReviewAvatar, req.Avatar)
	if err != nil {
		return nil, reviewError(err)
	}
	onChangeUsernameEvent(user.ID, user.Username)
	return &web.ChangeAvatarResp{Pending: result.Pending}, nil
}

func (s *coreSrv) TweetCollectionStatus(req *web.TweetCollectionStatusReq) (*web.TweetCollectionStatusResp, error) {
	resp := &web.TweetCollectionStatusResp{
		Status: true,
	}
	if _, err := s.Ds.GetUserPostCollection(req.TweetId, req.Uid); err != nil {
		resp.Status = false
		return resp, nil
	}
	return resp, nil
}

func (s *coreSrv) TweetStarStatus(req *web.TweetStarStatusReq) (*web.TweetStarStatusResp, error) {
	resp := &web.TweetStarStatusResp{
		Status: true,
	}
	if _, err := s.Ds.GetUserPostStar(req.TweetId, req.Uid); err != nil {
		resp.Status = false
		return resp, nil
	}
	return resp, nil
}

func (s *coreSrv) messagesFromCache(req *web.GetMessagesReq, limit int, offset int) (res *web.GetMessagesResp, key string, ok bool) {
	key = fmt.Sprintf("%s%d:%s:%d:%d", s.prefixMessages, req.Uid, req.Style, limit, offset)
	if data, err := s.wc.Get(key); err == nil {
		ok, res = true, &web.GetMessagesResp{
			CachePageResp: joint.CachePageResp{
				JsonResp: data,
			},
		}
	}
	return
}

func newCoreSrv(s *base.DaoServant, oss core.ObjectStorageService, wc core.WebCache) api.Core {
	cs := conf.CacheSetting
	return &coreSrv{
		DaoServant:     s,
		oss:            oss,
		wc:             wc,
		messagesExpire: cs.MessagesExpire,
		prefixMessages: conf.PrefixMessages,
	}
}
