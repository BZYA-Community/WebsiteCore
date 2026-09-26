// Copyright 2023 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package conf

import (
	"fmt"
	"sync"

	"github.com/redis/rueidis"
	"github.com/sirupsen/logrus"
)

var (
	_redisClient    rueidis.Client
	_redisClientErr error
	_onceRedis      sync.Once
)

// RedisClient 返回共享的 redis 客户端，首次调用时创建；失败时返回 error 而不是杀进程。
func RedisClient() (rueidis.Client, error) {
	_onceRedis.Do(func() {
		client, err := rueidis.NewClient(rueidis.ClientOption{
			InitAddress:      redisSetting.InitAddress,
			Username:         redisSetting.Username,
			Password:         redisSetting.Password,
			SelectDB:         redisSetting.SelectDB,
			ConnWriteTimeout: redisSetting.ConnWriteTimeout,
		})
		if err != nil {
			_redisClientErr = fmt.Errorf("create a redis client failed: %w", err)
			return
		}
		_redisClient = client
		// 顺便初始化一下CacheKeyPool
		initCacheKeyPool()
	})
	return _redisClient, _redisClientErr
}

// MustRedisClient 返回共享的 redis 客户端，初始化失败时记录错误并返回 nil。
// 需要处理初始化错误的调用方（如 cmd 层）请改用 RedisClient。
func MustRedisClient() rueidis.Client {
	client, err := RedisClient()
	if err != nil {
		logrus.Error(err)
	}
	return client
}
