package dbr

import "errors"

var (
	ErrPermission      = errors.New("没有执行此操作的权限")
	ErrTeacherInUse    = errors.New("请先取消 Mentor 标记并转移全部课程")
	ErrWhisperPhone    = errors.New("请先绑定手机号后再发送私信")
	ErrWhisperIdentity = errors.New("当前双方身份不允许发送私信")
	ErrWhisperPending  = errors.New("请等待对方回复后再发送私信")
	ErrWhisperBlocked  = errors.New("此会话已被屏蔽")
)

// WhisperConversation survives identity changes and blocking. It never infers
// permission from message history (a cancelled request must remain cancelled).
type WhisperConversation struct {
	LowUserID       int64 `gorm:"primaryKey"`
	HighUserID      int64 `gorm:"primaryKey"`
	PendingSenderID *int64
	Established     bool
	AdminContacted  bool
	BlockedByLow    bool
	BlockedByHigh   bool
}

func (c *WhisperConversation) PairAllowed(a, b *User) bool {
	if !a.IsActive() || b == nil || b.Model == nil || b.IsDel != 0 {
		return false
	}
	if a.IsAdminLevel() {
		return true
	}
	if !b.IsActive() {
		return false
	}
	if b.IsAdminLevel() {
		return true
	}
	return (a.IsStudent() || a.IsTeacher()) && (b.IsStudent() || b.IsTeacher()) && (a.IsTeacher() || b.IsTeacher())
}

func (c *WhisperConversation) CanRequest(sender, receiver *User) bool {
	if !c.PairAllowed(sender, receiver) {
		return false
	}
	return sender.IsAdminLevel() || sender.IsTeacher() || receiver.IsTeacher()
}

func (c *WhisperConversation) CanSend(sender, receiver *User) error {
	if sender == nil || receiver == nil || sender.Model == nil || receiver.Model == nil || sender.ID == receiver.ID {
		return ErrWhisperIdentity
	}
	if !c.PairAllowed(sender, receiver) {
		return ErrWhisperIdentity
	}
	if sender.Phone == "" {
		return ErrWhisperPhone
	}
	// Administrative contact cannot be blocked by members.
	if sender.IsAdminLevel() {
		return nil
	}
	if !receiver.IsAdminLevel() && (c.BlockedByLow || c.BlockedByHigh) {
		return ErrWhisperBlocked
	}
	if sender.IsStudent() && receiver.IsAdminLevel() && !c.AdminContacted {
		return ErrWhisperIdentity
	}
	if c.Established {
		return nil
	}
	if c.PendingSenderID != nil {
		if *c.PendingSenderID == sender.ID {
			return ErrWhisperPending
		}
		return nil // The receiver accepts the request by replying.
	}
	if !c.CanRequest(sender, receiver) {
		return ErrWhisperIdentity
	}
	return nil
}

func (c *WhisperConversation) Sent(sender *User) {
	if sender.IsAdminLevel() {
		c.AdminContacted, c.Established, c.PendingSenderID = true, true, nil
	} else if c.PendingSenderID != nil && *c.PendingSenderID != sender.ID {
		c.Established, c.PendingSenderID = true, nil
	} else if !c.Established {
		id := sender.ID
		c.PendingSenderID = &id
	}
}
