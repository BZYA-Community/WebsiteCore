// Copyright 2023 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package cache

import (
	"bytes"
	"encoding/gob"
	"time"

	"github.com/BZYA-Community/WebsiteCore/internal/conf"
	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/core/cs"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/RoaringBitmap/roaring/roaring64"
	"github.com/sirupsen/logrus"
)

const (
	// sendRetryMax 通道满时的发送重试次数上限
	sendRetryMax = 3
	// sendRetryInterval 通道满时的发送重试间隔
	sendRetryInterval = 10 * time.Millisecond
)

// trySend 以非阻塞方式尝试发送 item 到 ch，通道满时做有限次退避重试，
// 仍失败则丢弃并记录告警日志（缓存可重建，避免无界拉起协程造成泄漏）。
func trySend[T any](ch chan<- T, quit <-chan struct{}, item T, tag string) bool {
	for i := 1; i <= sendRetryMax; i++ {
		select {
		case ch <- item:
			return true
		case <-quit:
			logrus.Debugf("%s stopped, drop item", tag)
			return false
		case <-time.After(sendRetryInterval):
			logrus.Debugf("%s channel full, retry %d/%d", tag, i, sendRetryMax)
		}
	}
	logrus.Warnf("%s channel is full after %d retries, drop item", tag, sendRetryMax)
	return false
}

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
	// 先从缓存获取， 不处理错误
	key := conf.KeyUserInfoById.Get(id)
	if data, xerr := s.ac.Get(key); xerr == nil {
		buf := bytes.NewBuffer(data)
		res = &ms.User{}
		err = gob.NewDecoder(buf).Decode(res)
		return
	}
	// 最后查库
	if res, err = s.DataService.GetUserByID(id); err == nil {
		// 更新缓存
		onCacheUserInfoEvent(key, res)
	}
	return
}

func (s *cacheDataService) GetUserByUsername(username string) (res *ms.User, err error) {
	// 先从缓存获取， 不处理错误
	key := conf.KeyUserInfoByName.Get(username)
	if data, xerr := s.ac.Get(key); xerr == nil {
		buf := bytes.NewBuffer(data)
		res = &ms.User{}
		err = gob.NewDecoder(buf).Decode(res)
		return
	}
	// 最后查库
	if res, err = s.DataService.GetUserByUsername(username); err == nil {
		// 更新缓存
		onCacheUserInfoEvent(key, res)
	}
	return
}

func (s *cacheDataService) UserProfileByName(username string) (res *cs.UserProfile, err error) {
	// 先从缓存获取， 不处理错误
	key := conf.KeyUserProfileByName.Get(username)
	if data, xerr := s.ac.Get(key); xerr == nil {
		buf := bytes.NewBuffer(data)
		res = &cs.UserProfile{}
		err = gob.NewDecoder(buf).Decode(res)
		return
	}
	// 最后查库
	if res, err = s.DataService.UserProfileByName(username); err == nil {
		// 更新缓存
		onCacheObjectEvent(key, res, conf.CacheSetting.UserProfileExpire)
	}
	return
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
