package web

import (
	"testing"

	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
)

func TestFollowNotificationIdentifiesSender(t *testing.T) {
	for _, tc := range []struct{ nickname, want string }{
		{nickname: "小明", want: "小明关注了你"},
		{want: "follower关注了你"},
	} {
		t.Run(tc.want, func(t *testing.T) {
			user := &ms.User{Model: &ms.Model{ID: 12}, Username: "follower", Nickname: tc.nickname}
			message := followNotification(user, 34)
			if message.Brief != tc.want {
				t.Fatalf("notification preview = %q, want %q", message.Brief, tc.want)
			}
			if message.SenderUserID != 12 || message.ReceiverUserID != 34 || message.Type != ms.MsgTypeSystem {
				t.Fatalf("notification lost sender, recipient or system type: %+v", message)
			}
		})
	}
}
