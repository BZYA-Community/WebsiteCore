// Copyright 2023 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package cache

import (
	"github.com/BZYA-Community/WebsiteCore/internal/conf"
	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/core/cs"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/RoaringBitmap/roaring/roaring64"
)

type cacheDataService struct {
	core.DataService
	ac core.AppCache
}

func NewCacheDataService(ds core.DataService) core.DataService {
	lazyInitial()
	return &cacheDataService{
		DataService: ds,
		ac:          _appCache,
	}
}

func (s *cacheDataService) GetUserByID(id int64) (res *ms.User, err error) {
	// Identity, status and password revocation must take effect immediately.
	return s.DataService.GetUserByID(id)
}

func (s *cacheDataService) GetUserByUsername(username string) (res *ms.User, err error) {
	return s.DataService.GetUserByUsername(username)
}

func (s *cacheDataService) UserProfileByName(username string) (res *cs.UserProfile, err error) {
	return s.DataService.UserProfileByName(username)
}

func (s *cacheDataService) IsMyFollow(userId int64, followIds ...int64) (res map[int64]bool, err error) {
	size := len(followIds)
	res = make(map[int64]bool, size)
	if size == 0 {
		return
	}
	// 从缓存中获取
	key := conf.KeyMyFollowIds.Get(userId)
	if data, xerr := s.ac.Get(key); xerr == nil {
		bitmap := roaring64.New()
		if err = bitmap.UnmarshalBinary(data); err == nil {
			for _, followId := range followIds {
				res[followId] = bitmap.Contains(uint64(followId))
			}
			return
		}
	}
	// 直接查库并触发缓存更新事件
	OnCacheMyFollowIdsEvent(s.DataService, userId, key)
	return s.DataService.IsMyFollow(userId, followIds...)
}
