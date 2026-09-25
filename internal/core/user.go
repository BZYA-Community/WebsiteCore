// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package core

import (
	"github.com/BZYA-Community/WebsiteCore/internal/core/cs"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
)

// UserManageService 用户管理服务
type UserManageService interface {
	GetUserByID(id int64) (*ms.User, error)
	GetUserByUsername(username string) (*ms.User, error)
	GetUserByPhone(phone string) (*ms.User, error)
	GetUsersByIDs(ids []int64) ([]*ms.User, error)
	GetUsersByKeyword(keyword string) ([]*ms.User, error)
	UserProfileByName(username string) (*cs.UserProfile, error)
	CreateUser(user *ms.User) (*ms.User, error)
	UpdateUser(user *ms.User) error
	GetRegisterUserCount() (int64, error)
}

// FollowingManageService 关注管理服务
type FollowingManageService interface {
	FollowUser(userId int64, followId int64) error
	UnfollowUser(userId int64, followId int64) error
	ListFollows(userId int64, limit, offset int) (*ms.ContactList, error)
	ListFollowings(userId int64, limit, offset int) (*ms.ContactList, error)
	GetFollowCount(userId int64) (int64, int64, error)
	IsFollow(userId int64, followId int64) bool
}

// UserRelationService 用户关系服务
type UserRelationService interface {
	MyFollowIds(userId int64) ([]int64, error)
	IsMyFollow(userId int64, followIds ...int64) (map[int64]bool, error)
}
