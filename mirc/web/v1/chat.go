package v1

import (
	. "github.com/alimy/mir/v5"

	"github.com/BZYA-Community/WebsiteCore/internal/model/web"
)

// Chat 站内私聊服务，需要授权访问
type Chat struct {
	Schema `mir:"v1,chain"`

	// GetChatContacts 私信会话列表(含系统联系人)
	GetChatContacts func(Get, web.GetChatContactsReq) web.GetChatContactsResp `mir:"user/chat/contacts"`

	// GetChatHistory 聊天历史(user_id=0 为系统会话, 打开即读)
	GetChatHistory func(Get, web.GetChatHistoryReq) web.GetChatHistoryResp `mir:"user/chat/history"`

	// SendChatMessage 发送私信(身份组权限后端强制)
	SendChatMessage func(Post, web.SendChatMessageReq) web.SendChatMessageResp `mir:"user/chat/send"`
}
