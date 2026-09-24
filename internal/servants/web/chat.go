// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package web

import (
	"context"
	"crypto/md5"
	"fmt"
	"strings"
	"unicode/utf8"

	api "github.com/BZYA-Community/WebsiteCore/auto/api/v1"
	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/core/cs"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/jinzhu/dbr"
	"github.com/BZYA-Community/WebsiteCore/internal/model/web"
	"github.com/BZYA-Community/WebsiteCore/internal/servants/base"
	"github.com/BZYA-Community/WebsiteCore/internal/servants/chain"
	"github.com/BZYA-Community/WebsiteCore/pkg/xerror"
	"github.com/gin-gonic/gin"
	"github.com/redis/rueidis"
	"github.com/sirupsen/logrus"
)

// _maxWhisperContentLen 单条私信上限(字)
const _maxWhisperContentLen = 500

type chatSrv struct {
	api.UnimplementedChatServant
	*base.DaoServant
	ac core.AppCache
}

func (s *chatSrv) Chain() gin.HandlersChain {
	return gin.HandlersChain{chain.JWT()}
}

// canWhisper 身份组私信权限判定(发送入口与 history can_send 共用):
// 游客(未绑手机)禁止发送; 道友↔道友绝对禁止(好友不豁免);
// 道友→高级身份(导师/审核/管理/运维)B站式首条限制——对方回复前只能发一条;
// 高级身份→任何人 自由。返回 nil 表示允许, 否则为具体拒绝原因(msg 即提示文案)
func (s *chatSrv) canWhisper(sender, receiver *ms.User) *xerror.Error {
	if sender.Phone == "" {
		return web.ErrWhisperGuestNeedPhone
	}
	if sender.HasAnyRole() {
		return nil
	}
	// 发送方为道友
	if receiver.HasAnyRole() {
		if has, _ := s.Ds.HasWhispered(receiver.ID, sender.ID); has {
			// 对方回复过, 解除限制
			return nil
		}
		if has, _ := s.Ds.HasWhispered(sender.ID, receiver.ID); has {
			return web.ErrWhisperOnePending
		}
		return nil
	}
	if receiver.Phone == "" {
		return web.ErrWhisperPeerNoPhone
	}
	return web.ErrWhisperBetweenDaoyou
}

// GetChatContacts 会话列表(系统联系人单列置顶, 私信会话按最新消息倒序)
func (s *chatSrv) GetChatContacts(req *web.GetChatContactsReq) (*web.GetChatContactsResp, error) {
	resp := &web.GetChatContactsResp{
		System:   web.ChatContactItem{UserID: 0, Nickname: "系统通知", Avatar: "/logo.png"},
		Contacts: []web.ChatContactItem{},
	}
	// 系统会话: 未读数 + 最近一条摘要
	if unread, err := s.Ds.CountSystemUnread(req.Uid); err == nil {
		resp.System.Unread = unread
	}
	if msgs, _, err := s.Ds.GetMessages(req.Uid, cs.StyleMsgSystem, 1, 0); err == nil && len(msgs) > 0 {
		last := msgs[0]
		resp.System.LastTime = last.CreatedOn
		resp.System.LastContent = last.Brief
		if resp.System.LastContent == "" {
			resp.System.LastContent = last.Content
		}
	}

	// 私信会话: 最近行按对端分组(行序 id DESC, 首次出现即该会话最新一条)
	rows, err := s.Ds.GetRecentWhispers(req.Uid, 500)
	if err != nil {
		logrus.Errorf("Ds.GetRecentWhispers err: %s", err)
		return nil, web.ErrGetMessagesFailed
	}
	unreadBySender, err := s.Ds.CountWhisperUnreadBySender(req.Uid)
	if err != nil {
		logrus.Errorf("Ds.CountWhisperUnreadBySender err: %s", err)
		return nil, web.ErrGetMessagesFailed
	}
	seen := make(map[int64]bool, len(rows))
	otherIDs := make([]int64, 0, len(rows))
	for _, m := range rows {
		otherID := m.SenderUserID
		if otherID == req.Uid {
			otherID = m.ReceiverUserID
		}
		if !seen[otherID] {
			seen[otherID] = true
			otherIDs = append(otherIDs, otherID)
		}
	}
	users, err := s.Ds.GetUsersByIDs(otherIDs)
	if err != nil {
		logrus.Errorf("Ds.GetUsersByIDs err: %s", err)
		return nil, web.ErrGetMessagesFailed
	}
	userMap := make(map[int64]*ms.User, len(users))
	for _, u := range users {
		userMap[u.ID] = u
	}
	for _, m := range rows {
		otherID := m.SenderUserID
		if otherID == req.Uid {
			otherID = m.ReceiverUserID
		}
		if seen[otherID] {
			seen[otherID] = false // 只取首次出现(最新一条)
			u := userMap[otherID]
			if u == nil {
				continue
			}
			resp.Contacts = append(resp.Contacts, web.ChatContactItem{
				UserID:      u.ID,
				Username:    u.Username,
				Nickname:    u.Nickname,
				Avatar:      u.Avatar,
				Roles:       u.RoleList(),
				Identity:    dbr.IdentityOf(u.Roles, u.Phone),
				LastContent: m.Content,
				LastTime:    m.CreatedOn,
				LastFromMe:  m.SenderUserID == req.Uid,
				Unread:      unreadBySender[otherID],
			})
		}
	}
	return resp, nil
}

// GetChatHistory 聊天历史(user_id=0 系统会话只读; 打开即读)
func (s *chatSrv) GetChatHistory(req *web.GetChatHistoryReq) (*web.GetChatHistoryResp, error) {
	limit, offset := req.PageSize, (req.Page-1)*req.PageSize
	if limit <= 0 || limit > 100 {
		limit = 20
		offset = 0
	}
	resp := &web.GetChatHistoryResp{
		Messages: []web.ChatHistoryItem{},
		CanSend:  true,
	}
	if req.UserID == 0 {
		// 系统会话: @/评论/回复/审核/关注等通知, 只读
		msgs, total, err := s.Ds.GetMessages(req.Uid, cs.StyleMsgSystem, limit, offset)
		if err != nil {
			logrus.Errorf("Ds.GetMessages(system) err: %s", err)
			return nil, web.ErrGetMessagesFailed
		}
		senderInfo := s.batchSenderInfo(msgs)
		// id DESC 翻转为时间正序
		for i := len(msgs) - 1; i >= 0; i-- {
			m := msgs[i]
			info := senderInfo[m.SenderUserID]
			resp.Messages = append(resp.Messages, web.ChatHistoryItem{
				ID:             m.ID,
				SenderID:       m.SenderUserID,
				SenderName:     info.nickname,
				SenderUsername: info.username,
				Brief:          m.Brief,
				Content:        m.Content,
				Type:           int8(m.Type),
				PostID:         m.PostID,
				CommentID:      m.CommentID,
				Timestamp:      m.CreatedOn,
				Seen:           true,
			})
		}
		resp.TotalRows = total
		resp.CanSend = false
		resp.CanSendTip = "系统通知不支持回复"
		// 打开系统会话即全部置已读
		if err := s.Ds.ReadSystemMessages(req.Uid); err != nil {
			logrus.Errorf("Ds.ReadSystemMessages err: %s", err)
		}
		onMessageActionEvent(_messageActionRead, req.Uid)
		return resp, nil
	}

	// 私信会话
	peer, err := s.Ds.GetUserByID(req.UserID)
	if err != nil || peer.Model == nil || peer.ID <= 0 {
		return nil, web.ErrNoExistUsername
	}
	rows, total, err := s.Ds.GetWhisperHistory(req.Uid, peer.ID, limit, offset)
	if err != nil {
		logrus.Errorf("Ds.GetWhisperHistory err: %s", err)
		return nil, web.ErrGetMessagesFailed
	}
	for i := len(rows) - 1; i >= 0; i-- {
		m := rows[i]
		resp.Messages = append(resp.Messages, web.ChatHistoryItem{
			ID:        m.ID,
			SenderID:  m.SenderUserID,
			Content:   m.Content,
			Type:      int8(m.Type),
			Timestamp: m.CreatedOn,
			Seen:      m.SenderUserID == req.Uid && m.IsRead == 1,
		})
	}
	resp.TotalRows = total
	resp.Peer = &web.ChatContactItem{
		UserID:   peer.ID,
		Username: peer.Username,
		Nickname: peer.Nickname,
		Avatar:   peer.Avatar,
		Roles:    peer.RoleList(),
		Identity: dbr.IdentityOf(peer.Roles, peer.Phone),
	}
	// 打开会话即标记对方发来的未读
	if err := s.Ds.ReadWhispersFrom(req.Uid, peer.ID); err != nil {
		logrus.Errorf("Ds.ReadWhispersFrom err: %s", err)
	}
	onMessageActionEvent(_messageActionRead, req.Uid)
	// 附带发送权限(前端输入框引导, 后端发送时仍会强制校验)
	if xerr := s.canWhisper(req.User, peer); xerr != nil {
		resp.CanSend = false
		resp.CanSendTip = xerr.Msg()
	}
	return resp, nil
}

// batchSenderInfo 批量取消息发送者昵称/用户名(系统会话展示与跳转用, 0 → 系统)
func (s *chatSrv) batchSenderInfo(msgs []*ms.MessageFormated) map[int64]struct{ nickname, username string } {
	ids := make([]int64, 0, len(msgs))
	seen := map[int64]bool{}
	for _, m := range msgs {
		if m.SenderUserID > 0 && !seen[m.SenderUserID] {
			seen[m.SenderUserID] = true
			ids = append(ids, m.SenderUserID)
		}
	}
	info := map[int64]struct{ nickname, username string }{
		0: {nickname: "系统"},
	}
	if len(ids) == 0 {
		return info
	}
	users, err := s.Ds.GetUsersByIDs(ids)
	if err != nil {
		logrus.Errorf("Ds.GetUsersByIDs err: %s", err)
		return info
	}
	for _, u := range users {
		info[u.ID] = struct{ nickname, username string }{nickname: u.Nickname, username: u.Username}
	}
	return info
}

// SendChatMessage 发送私信(身份组权限后端强制)
func (s *chatSrv) SendChatMessage(req *web.SendChatMessageReq) (*web.SendChatMessageResp, error) {
	// 系统会话只读
	if req.UserID == 0 {
		return nil, web.ErrSystemChatReadonly
	}
	if req.User.ID == req.UserID {
		return nil, web.ErrNoWhisperToSelf
	}
	// 内容校验
	content := strings.TrimSpace(req.Content)
	if content == "" {
		return nil, web.ErrSendWhisperFailed.WithDetails("私信内容不能为空")
	}
	if utf8.RuneCountInString(content) > _maxWhisperContentLen {
		return nil, web.ErrSendWhisperFailed.WithDetails(fmt.Sprintf("私信内容不能超过%d字", _maxWhisperContentLen))
	}
	// 接收人存在性
	receiver, err := s.Ds.GetUserByID(req.UserID)
	if err != nil || receiver.Model == nil || receiver.ID <= 0 {
		return nil, web.ErrNoExistUsername
	}
	// 身份组权限
	if xerr := s.canWhisper(req.User, receiver); xerr != nil {
		return nil, xerr
	}
	ctx := context.Background()
	// 防重复发送: 同人同内容10秒窗口去重
	dedupKey := fmt.Sprintf("paopao:chat:dedup:%d:%d:%x", req.User.ID, receiver.ID, md5.Sum([]byte(content)))
	if err := s.ac.SetNx(dedupKey, []byte{1}, 10); rueidis.IsRedisNil(err) {
		return nil, web.ErrDuplicateWhisper
	}
	// 今日频次限制
	if count, _ := s.Redis.GetCountWhisper(ctx, req.User.ID); count >= _maxWhisperNumDaily {
		return nil, web.ErrTooManyWhisperNum
	}
	// 创建私信
	msg, err := s.Ds.CreateMessage(&ms.Message{
		SenderUserID:   req.User.ID,
		ReceiverUserID: receiver.ID,
		Type:           ms.MsgTypeWhisper,
		Brief:          "给你发送新私信了",
		Content:        content,
	})
	if err != nil {
		logrus.Errorf("Ds.CreateMessage(whisper) err: %s", err)
		return nil, web.ErrSendWhisperFailed
	}
	// 缓存处理, 不需要处理错误
	onMessageActionEvent(_messageActionSendWhisper, req.User.ID, receiver.ID)
	// 写入当日（自然日）计数缓存
	s.Redis.IncrCountWhisper(ctx, req.User.ID)

	return &web.SendChatMessageResp{MessageID: msg.ID}, nil
}

func newChatSrv(s *base.DaoServant, ac core.AppCache) api.Chat {
	return &chatSrv{
		DaoServant: s,
		ac:         ac,
	}
}
