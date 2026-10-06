// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package search

import (
	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
)

type tweetSearchFilter struct{}

func (s *tweetSearchFilter) filterResp(user *ms.User, resp *core.QueryResp) {
	if user != nil && !user.HasPermission("post.view") {
		resp.Items, resp.Total = []*ms.PostFormated{}, 0
		return
	}
	// Only explicit private-content access bypasses visibility filtering.
	if user != nil && user.HasPermission("content.view_private") {
		return
	}

	var item *ms.PostFormated
	items := resp.Items
	latestIndex := len(items) - 1
	if user == nil {
		for i := 0; i <= latestIndex; i++ {
			item = items[i]
			if item.Visibility != core.PostVisitPublic {
				items[i] = items[latestIndex]
				items = items[:latestIndex]
				resp.Total--
				latestIndex--
				i--
			}
		}
	} else {
		var cutPrivate bool
		for i := 0; i <= latestIndex; i++ {
			item = items[i]
			// 好友功能已移除: 好友可见与私密同口径(仅作者本人可见)
			cutPrivate = ((item.Visibility == core.PostVisitPrivate || item.Visibility == core.PostVisitFriend) && user.ID != item.UserID)
			if cutPrivate {
				items[i] = items[latestIndex]
				items = items[:latestIndex]
				resp.Total--
				latestIndex--
				i--
			}
		}
	}

	resp.Items = items
}
