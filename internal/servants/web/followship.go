// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package web

import (
	api "github.com/BZYA-Community/WebsiteCore/auto/api/v1"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/cache"
	"github.com/BZYA-Community/WebsiteCore/internal/model/web"
	"github.com/BZYA-Community/WebsiteCore/internal/servants/base"
	"github.com/BZYA-Community/WebsiteCore/internal/servants/chain"
	"github.com/BZYA-Community/WebsiteCore/pkg/xerror"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type followshipSrv struct {
	api.UnimplementedFollowshipServant
	*base.DaoServant
}

func (s *followshipSrv) Chain() gin.HandlersChain {
	return gin.HandlersChain{chain.JwtLoose()}
}

func (s *followshipSrv) ListFollowings(r *web.ListFollowingsReq) (*web.ListFollowingsResp, error) {
	he, err := s.Ds.GetUserByUsername(r.Username)
	if err != nil {
		logrus.Errorf("Ds.GetUserByUsername err: %s", err)
		return nil, web.ErrNoExistUsername
	}
	res, err := s.Ds.ListFollowings(he.ID, r.PageSize, (r.Page-1)*r.PageSize)
	if err != nil {
		logrus.Errorf("Ds.ListFollowings err: %s", err)
		return nil, web.ErrListFollowingsFailed
	}
	if r.BaseInfo.User != nil {
		for i, contact := range res.Contacts {
			res.Contacts[i].IsFollowing = s.Ds.IsFollow(r.User.ID, contact.UserId)
		}
	}
	resp := base.PageRespFrom(res.Contacts, r.Page, r.PageSize, res.Total)
	return (*web.ListFollowingsResp)(resp), nil
}

func (s *followshipSrv) ListFollows(r *web.ListFollowsReq) (*web.ListFollowsResp, error) {
	he, err := s.Ds.GetUserByUsername(r.Username)
	if err != nil {
		logrus.Errorf("Ds.GetUserByUsername err: %s", err)
		return nil, web.ErrNoExistUsername
	}
	res, err := s.Ds.ListFollows(he.ID, r.PageSize, (r.Page-1)*r.PageSize)
	if err != nil {
		logrus.Errorf("Ds.ListFollows err: %s", err)
		return nil, web.ErrListFollowsFailed
	}
	if r.BaseInfo.User != nil {
		if r.User.Username == r.Username {
			for i := range res.Contacts {
				res.Contacts[i].IsFollowing = true
			}
		} else {
			for i, contact := range res.Contacts {
				res.Contacts[i].IsFollowing = s.Ds.IsFollow(r.User.ID, contact.UserId)
			}
		}
	}
	resp := base.PageRespFrom(res.Contacts, r.Page, r.PageSize, res.Total)
	return (*web.ListFollowsResp)(resp), nil
}

func (s *followshipSrv) UnfollowUser(r *web.UnfollowUserReq) error {
	if r.User == nil {
		return xerror.UnauthorizedTokenError
	} else if r.User.ID == r.UserId {
		return web.ErrNotAllowUnfollowSelf
	}
	if err := s.Ds.UnfollowUser(r.User.ID, r.UserId); err != nil {
		logrus.Errorf("Ds.UnfollowUser err: %s userId: %d followId: %d", err, r.User.ID, r.UserId)
		return web.ErrUnfollowUserFailed
	}
	// 触发缓存更新事件
	// TODO: 合并成一个事件
	cache.OnCacheMyFollowIdsEvent(s.Ds, r.User.ID)
	cache.OnExpireIndexTweetEvent(r.User.ID)
	onMessageActionEvent(_messageActionFollow, r.User.ID)
	onTrendsActionEvent(_trendsActionUnfollowUser, r.User.ID)
	return nil
}

func (s *followshipSrv) FollowUser(r *web.FollowUserReq) error {
	if r.User == nil {
		return xerror.UnauthorizedTokenError
	} else if r.User.ID == r.UserId {
		return web.ErrNotAllowFollowSelf
	}
	if err := s.Ds.FollowUser(r.User.ID, r.UserId); err != nil {
		logrus.Errorf("Ds.FollowUser err: %s userId: %d followId: %d", err, r.User.ID, r.UserId)
		return web.ErrUnfollowUserFailed
	}
	// 关注通知(系统会话, 带关注者id便于前端跳转其主页)
	onCreateMessageEvent(&ms.Message{
		SenderUserID:   r.User.ID,
		ReceiverUserID: r.UserId,
		Type:           ms.MsgTypeSystem,
		Brief:          "关注了你",
		Content:        "快去看看 TA 的主页吧",
	})
	// 触发缓存更新事件
	// TODO: 合并成一个事件
	cache.OnCacheMyFollowIdsEvent(s.Ds, r.User.ID)
	cache.OnExpireIndexTweetEvent(r.User.ID)
	onMessageActionEvent(_messageActionFollow, r.User.ID)
	onTrendsActionEvent(_trendsActionFollowUser, r.User.ID)
	return nil
}

func newFollowshipSrv(s *base.DaoServant) api.Followship {
	return &followshipSrv{
		DaoServant: s,
	}
}
