// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package web

import (
	"errors"
	"testing"

	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/internal/model/joint"
	"github.com/BZYA-Community/WebsiteCore/internal/model/web"
)

// fakeDataService 是 core.DataService 的手写假实现(#18)。
//
// 内嵌 nil 接口让未覆写的方法一旦被调用即 panic, 便于发现被测代码意料之外的依赖;
// 被测仆人真正用到的方法用函数字段按需覆写。
// 整个测试不依赖 config.yaml / conf.Initial / dao 包级全局, 干净检出即可运行。
type fakeDataService struct {
	core.DataService

	getUserByUsername func(username string) (*ms.User, error)
	listFollowings    func(userId int64, limit, offset int) (*ms.ContactList, error)
	listFollows       func(userId int64, limit, offset int) (*ms.ContactList, error)
	isFollow          func(userId int64, followId int64) bool
}

func (f *fakeDataService) GetUserByUsername(username string) (*ms.User, error) {
	if f.getUserByUsername == nil {
		panic("fakeDataService.GetUserByUsername: unexpected call")
	}
	return f.getUserByUsername(username)
}

func (f *fakeDataService) ListFollowings(userId int64, limit, offset int) (*ms.ContactList, error) {
	if f.listFollowings == nil {
		panic("fakeDataService.ListFollowings: unexpected call")
	}
	return f.listFollowings(userId, limit, offset)
}

func (f *fakeDataService) ListFollows(userId int64, limit, offset int) (*ms.ContactList, error) {
	if f.listFollows == nil {
		panic("fakeDataService.ListFollows: unexpected call")
	}
	return f.listFollows(userId, limit, offset)
}

func (f *fakeDataService) IsFollow(userId int64, followId int64) bool {
	if f.isFollow == nil {
		panic("fakeDataService.IsFollow: unexpected call")
	}
	return f.isFollow(userId, followId)
}

// newUser 造一个带 ID 的 ms.User(内嵌 *Model 必须非 nil, 否则访问 ID 会空指针)。
func newUser(id int64, username string) *ms.User {
	return &ms.User{
		Model:    &ms.Model{ID: id},
		Username: username,
	}
}

// TestListFollowingsGuestPaging 覆盖游客视角的粉丝列表: 分页参数正确换算,
// 且游客(User == nil)不触发 IsFollow 查询。
func TestListFollowingsGuestPaging(t *testing.T) {
	isFollowCalled := false
	ds := &fakeDataService{
		getUserByUsername: func(username string) (*ms.User, error) {
			if username != "alice" {
				t.Fatalf("GetUserByUsername(%q), want alice", username)
			}
			return newUser(7, "alice"), nil
		},
		listFollowings: func(userId int64, limit, offset int) (*ms.ContactList, error) {
			if userId != 7 {
				t.Fatalf("ListFollowings(userId=%d), want 7", userId)
			}
			if limit != 20 || offset != 20 {
				t.Fatalf("ListFollowings(limit=%d, offset=%d), want (20, 20)", limit, offset)
			}
			return &ms.ContactList{
				Contacts: []ms.ContactItem{
					{UserId: 11, Username: "bob"},
					{UserId: 12, Username: "carol"},
				},
				Total: 5,
			}, nil
		},
		isFollow: func(int64, int64) bool {
			isFollowCalled = true
			return true
		},
	}

	s := NewFollowshipService(ds)
	resp, err := s.ListFollowings(&web.ListFollowingsReq{
		BasePageInfo: joint.BasePageInfo{Page: 2, PageSize: 20},
		Username:     "alice",
	})
	if err != nil {
		t.Fatalf("ListFollowings() error = %v", err)
	}
	if resp == nil {
		t.Fatal("ListFollowings() resp = nil")
	}
	if resp.Pager.Page != 2 || resp.Pager.PageSize != 20 || resp.Pager.TotalRows != 5 {
		t.Fatalf("pager = %+v, want {Page:2 PageSize:20 TotalRows:5}", resp.Pager)
	}
	contacts, ok := resp.List.([]ms.ContactItem)
	if !ok {
		t.Fatalf("resp.List type = %T, want []ms.ContactItem", resp.List)
	}
	if len(contacts) != 2 || contacts[0].Username != "bob" || contacts[1].Username != "carol" {
		t.Fatalf("contacts = %+v", contacts)
	}
	if contacts[0].IsFollowing || contacts[1].IsFollowing {
		t.Fatalf("guest 视角 IsFollowing 应保持 false: %+v", contacts)
	}
	if isFollowCalled {
		t.Fatal("guest 视角不应触发 IsFollow 查询")
	}
}

// TestListFollowingsUserNotExist 覆盖用户不存在的错误分支。
func TestListFollowingsUserNotExist(t *testing.T) {
	ds := &fakeDataService{
		getUserByUsername: func(string) (*ms.User, error) {
			return nil, errors.New("record not found")
		},
	}

	s := NewFollowshipService(ds)
	resp, err := s.ListFollowings(&web.ListFollowingsReq{
		BasePageInfo: joint.BasePageInfo{Page: 1, PageSize: 20},
		Username:     "ghost",
	})
	if resp != nil {
		t.Fatalf("ListFollowings() resp = %+v, want nil", resp)
	}
	if err != web.ErrNoExistUsername {
		t.Fatalf("ListFollowings() error = %v, want ErrNoExistUsername", err)
	}
}

// TestListFollowsMarksFollowingFlags 覆盖登录用户查看他人关注列表:
// 每个联系人的 IsFollowing 由 IsFollow(viewer, contact) 决定。
func TestListFollowsMarksFollowingFlags(t *testing.T) {
	type call struct{ userId, followId int64 }
	var calls []call
	ds := &fakeDataService{
		getUserByUsername: func(username string) (*ms.User, error) {
			if username != "alice" {
				t.Fatalf("GetUserByUsername(%q), want alice", username)
			}
			return newUser(7, "alice"), nil
		},
		listFollows: func(userId int64, limit, offset int) (*ms.ContactList, error) {
			if userId != 7 {
				t.Fatalf("ListFollows(userId=%d), want 7", userId)
			}
			return &ms.ContactList{
				Contacts: []ms.ContactItem{
					{UserId: 11, Username: "bob"},
					{UserId: 12, Username: "carol"},
				},
				Total: 2,
			}, nil
		},
		isFollow: func(userId int64, followId int64) bool {
			calls = append(calls, call{userId, followId})
			return followId == 11 // 只关注了 bob
		},
	}

	s := NewFollowshipService(ds)
	resp, err := s.ListFollows(&web.ListFollowsReq{
		BaseInfo:     web.BaseInfo{User: newUser(99, "me")},
		BasePageInfo: joint.BasePageInfo{Page: 1, PageSize: 20},
		Username:     "alice",
	})
	if err != nil {
		t.Fatalf("ListFollows() error = %v", err)
	}
	contacts, ok := resp.List.([]ms.ContactItem)
	if !ok {
		t.Fatalf("resp.List type = %T, want []ms.ContactItem", resp.List)
	}
	if len(contacts) != 2 {
		t.Fatalf("contacts len = %d, want 2", len(contacts))
	}
	if !contacts[0].IsFollowing || contacts[1].IsFollowing {
		t.Fatalf("IsFollowing = [%v %v], want [true false]", contacts[0].IsFollowing, contacts[1].IsFollowing)
	}
	if len(calls) != 2 || calls[0] != (call{99, 11}) || calls[1] != (call{99, 12}) {
		t.Fatalf("IsFollow calls = %+v, want [{99 11} {99 12}]", calls)
	}
}

// TestListFollowsSelfViewSkipsIsFollow 覆盖查看自己的关注列表的快捷分支:
// 全部标记为已关注且不打 IsFollow 查询。
func TestListFollowsSelfViewSkipsIsFollow(t *testing.T) {
	ds := &fakeDataService{
		getUserByUsername: func(username string) (*ms.User, error) {
			return newUser(7, username), nil
		},
		listFollows: func(userId int64, limit, offset int) (*ms.ContactList, error) {
			return &ms.ContactList{
				Contacts: []ms.ContactItem{
					{UserId: 11, Username: "bob", IsFollowing: false},
					{UserId: 12, Username: "carol", IsFollowing: false},
				},
				Total: 2,
			}, nil
		},
		// isFollow 未设置: 被调用即 panic, 即为断言。
	}

	s := NewFollowshipService(ds)
	resp, err := s.ListFollows(&web.ListFollowsReq{
		BaseInfo:     web.BaseInfo{User: newUser(7, "alice")},
		BasePageInfo: joint.BasePageInfo{Page: 1, PageSize: 20},
		Username:     "alice",
	})
	if err != nil {
		t.Fatalf("ListFollows() error = %v", err)
	}
	contacts, ok := resp.List.([]ms.ContactItem)
	if !ok {
		t.Fatalf("resp.List type = %T, want []ms.ContactItem", resp.List)
	}
	if len(contacts) != 2 || !contacts[0].IsFollowing || !contacts[1].IsFollowing {
		t.Fatalf("self 视角 IsFollowing 应全为 true: %+v", contacts)
	}
}
