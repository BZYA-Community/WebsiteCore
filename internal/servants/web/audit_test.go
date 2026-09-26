// Copyright 2023 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package web

import (
	"errors"
	"testing"

	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/jinzhu/dbr"
)

// fakeUserBatchDs 仅实现GetUsersByIDs
// 其余方法经嵌入的nil接口透传 调用即panic 以此断言usernamesOf只做一次批量查询
type fakeUserBatchDs struct {
	core.DataService
	calls  int
	gotIds []int64
	users  []*ms.User
	err    error
}

func (f *fakeUserBatchDs) GetUsersByIDs(ids []int64) ([]*ms.User, error) {
	f.calls++
	f.gotIds = append([]int64(nil), ids...)
	if f.err != nil {
		return nil, f.err
	}
	return f.users, nil
}

// TestUsernamesOfBatchLoad 验证usernamesOf一次WHERE id IN (?)取回:
// 去重并过滤非法id 缺失用户显示空 查询失败返回空表
func TestUsernamesOfBatchLoad(t *testing.T) {
	ds := &fakeUserBatchDs{
		users: []*ms.User{
			{Model: &dbr.Model{ID: 1}, Username: "alice"},
			{Model: &dbr.Model{ID: 3}, Username: "carol"},
		},
	}
	got := usernamesOf(ds, []int64{1, 1, 0, -2, 3, 9})
	if ds.calls != 1 {
		t.Fatalf("GetUsersByIDs calls = %d, want 1 (批量查询)", ds.calls)
	}
	wantIds := []int64{1, 3, 9}
	if len(ds.gotIds) != len(wantIds) {
		t.Fatalf("ids = %v, want %v (去重且过滤非法id)", ds.gotIds, wantIds)
	}
	for i, id := range wantIds {
		if ds.gotIds[i] != id {
			t.Fatalf("ids = %v, want %v", ds.gotIds, wantIds)
		}
	}
	if len(got) != 2 || got[1] != "alice" || got[3] != "carol" {
		t.Fatalf("got = %v, want {1:alice 3:carol}", got)
	}
	if name, exist := got[9]; exist {
		t.Fatalf("缺失用户不应出现在结果中: %q", name)
	}

	// 无有效id时不发起查询
	noIds := &fakeUserBatchDs{}
	if m := usernamesOf(noIds, []int64{0, -1}); len(m) != 0 {
		t.Fatalf("got = %v, want 空表", m)
	}
	if noIds.calls != 0 {
		t.Fatalf("GetUsersByIDs calls = %d, want 0", noIds.calls)
	}

	// 查询失败时返回空表(与逐个查询全部失败的兜底一致)
	failDs := &fakeUserBatchDs{err: errors.New("db down")}
	if m := usernamesOf(failDs, []int64{1}); len(m) != 0 {
		t.Fatalf("got = %v, want 空表", m)
	}
}
