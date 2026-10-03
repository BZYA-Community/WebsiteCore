// Package content assembles content views and applies the shared read policy.
// It accepts persistence at its seam and does not initialize backing services.
package content

import (
	"fmt"

	"github.com/BZYA-Community/WebsiteCore/internal/core/cs"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/pkg/types"
)

// Store is the subset of persistence needed to enrich content for a viewer.
// Production uses the cached GORM adapter; tests use an in-memory adapter.
type Store interface {
	IsMyFollow(userID int64, ids ...int64) (map[int64]bool, error)
	IsFollow(userID, authorID int64) bool
	GetPostByID(id int64) (*ms.Post, error)
	GetPostContentsByIDs(ids []int64) ([]*ms.PostContent, error)
	GetUsersByIDs(ids []int64) ([]*ms.User, error)
	GetUserByUsername(username string) (*ms.User, error)
}

// Views hides bulk enrichment, deleted authors, and the shared read policy.
// Prepare methods retain the existing in-place enrichment contract.
type Views struct{ store Store }

func New(store Store) *Views { return &Views{store: store} }

func (s *Views) PrepareUser(userId int64, user *ms.UserFormated) error {
	// guest用户的userId<0
	if userId < 0 {
		return nil
	}
	followMap, err := s.store.IsMyFollow(userId, user.ID)
	if err != nil {
		return err
	}
	user.IsFollowing = followMap[user.ID]
	return nil
}

func (s *Views) PrepareMessages(userId int64, messages []*ms.MessageFormated) error {
	// guest用户的userId<0
	if userId < 0 {
		return nil
	}
	userIds := make([]int64, 0, len(messages))
	for _, msg := range messages {
		if msg.SenderUser != nil {
			userIds = append(userIds, msg.SenderUserID)
		}
		if msg.ReceiverUser != nil {
			userIds = append(userIds, msg.ReceiverUserID)
		}
	}
	followMap, err := s.store.IsMyFollow(userId, userIds...)
	if err != nil {
		return err
	}
	for _, msg := range messages {
		if msg.SenderUser != nil {
			msg.SenderUser.IsFollowing = followMap[msg.SenderUserID]
		}
		if msg.ReceiverUser != nil {
			msg.ReceiverUser.IsFollowing = followMap[msg.ReceiverUserID]
		}
	}
	return nil
}

func (s *Views) PrepareTweet(user *ms.User, tweet *ms.PostFormated) error {
	// guest用户
	if user == nil {
		return nil
	}
	// 转换一下可见性的值
	tweet.Visibility = ms.PostVisibleT(tweet.Visibility.ToOutValue())
	followMap, err := s.store.IsMyFollow(user.ID, tweet.UserID)
	if err != nil {
		return err
	}
	tweet.User.IsFollowing = followMap[tweet.UserID]
	return nil
}

// CanViewTweet 统一校验用户对帖子的读权限(与TweetDetail的可见性判定保持同口径):
// 作者本人/管理员/审核员直接放行; 其余要求帖子已过审且满足可见性
// (公开 / 关注可见=访问者关注了作者), 私密帖仅作者与管理侧可见
// post 同时兼容 *ms.Post 与 *ms.PostFormated
func (s *Views) CanViewTweet(user *ms.User, post any) bool {
	var (
		userID   int64
		visible  ms.PostVisibleT
		audit    ms.PostAuditT
		hasValue bool
	)
	switch p := post.(type) {
	case *ms.Post:
		if p != nil {
			userID, visible, audit, hasValue = p.UserID, p.Visibility, p.AuditStatus, true
		}
	case *ms.PostFormated:
		if p != nil {
			userID, visible, audit, hasValue = p.UserID, p.Visibility, p.AuditStatus, true
		}
	default:
		return false
	}
	if !hasValue {
		return false
	}
	// 作者本人/管理员/审核员直接放行
	if user != nil && (user.ID == userID || user.IsAdmin || user.HasRole(ms.RoleAuditor)) {
		return true
	}
	// 其余情况要求帖子已过审
	if audit != ms.PostAuditApproved {
		return false
	}
	switch visible {
	case ms.PostVisitPublic:
		return true
	case ms.PostVisitFollowing:
		// 关注可见: 访问者关注了作者即可见(与TweetDetail的IsFollowing口径一致)
		return user != nil && s.store.IsFollow(user.ID, userID)
	default:
		// PostVisitPrivate及其它未知值一律拒绝
		return false
	}
}

func (s *Views) PrepareTweets(userId int64, tweets []*ms.PostFormated) error {
	userIdSet := make(map[int64]types.Empty, len(tweets))
	for _, tweet := range tweets {
		userIdSet[tweet.UserID] = types.Empty{}
		// 顺便转换一下可见性的值
		tweet.Visibility = ms.PostVisibleT(tweet.Visibility.ToOutValue())
	}
	// guest用户的userId<0
	if userId < 0 {
		return nil
	}
	userIds := make([]int64, 0, len(userIdSet))
	for id := range userIdSet {
		userIds = append(userIds, id)
	}
	followMap, err := s.store.IsMyFollow(userId, userIds...)
	if err != nil {
		return err
	}
	for _, tweet := range tweets {
		tweet.User.IsFollowing = followMap[tweet.UserID]
	}
	return nil
}

func (s *Views) GetTweetBy(id int64) (*ms.PostFormated, error) {
	post, err := s.store.GetPostByID(id)
	if err != nil {
		return nil, err
	}
	postContents, err := s.store.GetPostContentsByIDs([]int64{post.ID})
	if err != nil {
		return nil, err
	}
	users, err := s.store.GetUsersByIDs([]int64{post.UserID})
	if err != nil {
		return nil, err
	}
	// 数据整合
	postFormated := post.Format()
	for _, user := range users {
		postFormated.User = user.Format()
	}
	if postFormated.User == nil {
		// 作者用户已不存在时填充占位 避免前端空指针
		postFormated.User = ms.GhostUserFormated
	}
	for _, content := range postContents {
		if content.PostID == post.ID {
			postFormated.Contents = append(postFormated.Contents, content.Format())
		}
	}
	return postFormated, nil
}

func (s *Views) RelationTypFrom(me *ms.User, username string) (res *cs.VistUser, err error) {
	res = &cs.VistUser{
		RelTyp:   cs.RelationSelf,
		Username: username,
	}
	// visit by self
	if me != nil && me.Username == username {
		res.UserId = me.ID
		return
	}
	he, xerr := s.store.GetUserByUsername(username)
	if xerr != nil || (he.Model != nil && he.ID <= 0) {
		return nil, fmt.Errorf("get user failed with username: %s", username)
	}
	res.UserId = he.ID
	// visit by guest
	if me == nil {
		res.RelTyp = cs.RelationGuest
		return
	}
	// visit by admin/other(好友功能已移除 不存在好友关系)
	if me.IsAdmin {
		res.RelTyp = cs.RelationAdmin
	} else {
		res.RelTyp = cs.RelationGuest
	}
	return
}
