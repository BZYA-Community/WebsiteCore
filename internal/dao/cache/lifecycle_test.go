// Copyright 2023 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package cache

import (
	"runtime"
	"testing"
	"time"

	"github.com/BZYA-Community/WebsiteCore/internal/core"
)

// TestCacheIndexSrvCloseStopsConsumer 验证消费协程能随 Close 退出（不再泄漏）
func TestCacheIndexSrvCloseStopsConsumer(t *testing.T) {
	s := &cacheIndexSrv{
		indexActionCh: make(chan *core.IndexAction, 2),
		cachePostsCh:  make(chan *postsEntry, 2),
		quit:          make(chan struct{}),
	}
	done := make(chan struct{})
	go func() {
		s.startIndexPosts()
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("consumer should not exit before Close")
	case <-time.After(50 * time.Millisecond):
	}

	s.Close()
	s.Close() // 可重复调用

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("consumer did not exit after Close")
	}
}

// TestSimpleCacheIndexServantCloseStopsConsumer 验证 simple 索引协程与定时器随 Close 退出
func TestSimpleCacheIndexServantCloseStopsConsumer(t *testing.T) {
	s := &simpleCacheIndexServant{
		indexActionCh:   make(chan core.IdxAct, 2),
		checkTick:       time.NewTicker(time.Hour),
		expireIndexTick: time.NewTicker(time.Hour),
		quit:            make(chan struct{}),
	}
	done := make(chan struct{})
	go func() {
		s.startIndexPosts()
		close(done)
	}()

	s.Close()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("consumer did not exit after Close")
	}
}

// TestTrySendBounded 验证通道满时有限重试后丢弃，不拉起无界协程
func TestTrySendBounded(t *testing.T) {
	before := runtime.NumGoroutine()
	ch := make(chan int, 1)
	quit := make(chan struct{})
	ch <- 1

	start := time.Now()
	if trySend(ch, quit, 2, "test") {
		t.Fatal("trySend should drop item when channel is full")
	}
	if elapsed := time.Since(start); elapsed > time.Duration(sendRetryMax*20)*time.Millisecond {
		t.Fatalf("trySend blocked too long: %v", elapsed)
	}

	// 关闭后应立即放弃，不等待重试
	close(quit)
	<-ch // 清空通道后重新填满，模拟通道满且服务已停止
	ch <- 3
	start = time.Now()
	if trySend(ch, quit, 4, "test") {
		t.Fatal("trySend should drop item after quit")
	}
	if elapsed := time.Since(start); elapsed > 10*time.Millisecond {
		t.Fatalf("trySend should return immediately after quit, took %v", elapsed)
	}

	time.Sleep(50 * time.Millisecond)
	if after := runtime.NumGoroutine(); after > before+2 {
		t.Fatalf("trySend spawned goroutines: before %d, after %d", before, after)
	}
}
