// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package web

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/BZYA-Community/WebsiteCore/internal/core"
)

// fakeDeleteOSS 记录收到的DeleteObjects批次(仅实现删除路径用到的方法)
type fakeDeleteOSS struct {
	core.ObjectStorageService
	got chan []string
}

func newFakeDeleteOSS() *fakeDeleteOSS {
	return &fakeDeleteOSS{got: make(chan []string, 64)}
}

func (f *fakeDeleteOSS) ObjectKey(cUrl string) string {
	if idx := strings.Index(cUrl, ".com/"); idx >= 0 {
		return cUrl[idx+len(".com/"):]
	}
	return cUrl
}

func (f *fakeDeleteOSS) DeleteObject(objectKey string) error {
	f.got <- []string{objectKey}
	return nil
}

func (f *fakeDeleteOSS) DeleteObjects(objectKeys []string) error {
	f.got <- objectKeys
	return nil
}

var _ core.ObjectStorageService = (*fakeDeleteOSS)(nil)

// TestOssDeleteBatches 验证大列表按批次切分(共享底层数组, 不复制)
func TestOssDeleteBatches(t *testing.T) {
	cases := []struct {
		name string
		size int
		want int
	}{
		{"空列表", 0, 0},
		{"小于一批", 1, 1},
		{"等于一批", ossDeleteBatchSize, 1},
		{"整数倍", ossDeleteBatchSize * 3, 3},
		{"非整数倍", ossDeleteBatchSize*2 + 7, 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			keys := make([]string, tc.size)
			for i := range keys {
				keys[i] = fmt.Sprintf("key-%d", i)
			}
			batches := ossDeleteBatches(keys)
			if len(batches) != tc.want {
				t.Fatalf("ossDeleteBatches(%d keys) = %d batches, want %d", tc.size, len(batches), tc.want)
			}
			total := 0
			for _, batch := range batches {
				if len(batch) == 0 || len(batch) > ossDeleteBatchSize {
					t.Fatalf("batch size = %d, want 1..%d", len(batch), ossDeleteBatchSize)
				}
				// 批次共享原数组且顺序/内容不变
				if batch[0] != keys[total] {
					t.Fatalf("batch starts at %d, want %s", total, keys[total])
				}
				total += len(batch)
			}
			if total != tc.size {
				t.Fatalf("batched keys = %d, want %d", total, tc.size)
			}
		})
	}
}

// TestDeleteOssObjectsUsesBoundedPool 验证多键删除经有界worker池分批执行,
// 删除最终完成且每批不超过 ossDeleteBatchSize(#29)
func TestDeleteOssObjectsUsesBoundedPool(t *testing.T) {
	fake := newFakeDeleteOSS()
	const keyCount = ossDeleteBatchSize*2 + 7 // 207 键 → 3 批
	contents := make([]string, keyCount)
	for i := range contents {
		contents[i] = fmt.Sprintf("https://cdn.example.com/media/img-%d.png", i)
	}

	deleteOssObjects(fake, contents)

	wantBatches := (keyCount + ossDeleteBatchSize - 1) / ossDeleteBatchSize
	total, batches := 0, 0
	timeout := time.After(5 * time.Second)
	for batches < wantBatches {
		select {
		case keys := <-fake.got:
			if len(keys) == 0 || len(keys) > ossDeleteBatchSize {
				t.Fatalf("batch size = %d, want 1..%d", len(keys), ossDeleteBatchSize)
			}
			total += len(keys)
			batches++
		case <-timeout:
			t.Fatalf("timed out: got %d/%d batches (%d keys)", batches, wantBatches, total)
		}
	}
	if total != keyCount {
		t.Fatalf("deleted keys = %d, want %d", total, keyCount)
	}
}

// TestDeleteOssObjectsSingleKeyStillSynchronous 验证单键路径保持同步删除(不受池改动影响)
func TestDeleteOssObjectsSingleKeyStillSynchronous(t *testing.T) {
	fake := newFakeDeleteOSS()
	deleteOssObjects(fake, []string{"https://cdn.example.com/media/only.png"})
	select {
	case keys := <-fake.got:
		if len(keys) != 1 || keys[0] != "media/only.png" {
			t.Fatalf("DeleteObject key = %v, want [media/only.png]", keys)
		}
	case <-time.After(time.Second):
		t.Fatal("single-key delete was not executed synchronously")
	}
}
