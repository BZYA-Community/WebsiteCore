// Copyright 2023 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package ms

// GhostUserFormated 帖子/评论/回复的作者用户已不存在(历史数据/已清理)时的占位用户。
// 列表与详情接口在查不到作者用户行时填充该占位, 避免返回 null user
// 导致前端读取 avatar/nickname 等属性崩溃。
var GhostUserFormated = &UserFormated{
	ID:       -1,
	Nickname: "已注销用户",
	Username: "ghost",
	Avatar:   "https://paopao-demo.vercel.app/avatar/default/hanna.png",
}
