package jinzhu

import (
	"errors"

	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/jinzhu/dbr"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func conversationFor(a, b int64) *dbr.WhisperConversation {
	if a > b {
		a, b = b, a
	}
	return &dbr.WhisperConversation{LowUserID: a, HighUserID: b}
}

func getConversation(tx *gorm.DB, a, b int64, create bool) (*dbr.WhisperConversation, error) {
	c := conversationFor(a, b)
	if create {
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(c).Error; err != nil {
			return nil, err
		}
	}
	err := tx.Where("low_user_id = ? AND high_user_id = ?", c.LowUserID, c.HighUserID).First(c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) && !create {
		return c, nil
	}
	return c, err
}

func (s *messageSrv) WhisperPermission(senderID, receiverID int64) (*dbr.WhisperConversation, error) {
	var c *dbr.WhisperConversation
	var permission error
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := lockIdentityChanges(tx, false); err != nil {
			return err
		}
		users, err := lockUsers(tx, senderID, receiverID)
		if err != nil {
			return err
		}
		c, err = getConversation(tx, senderID, receiverID, false)
		if err != nil {
			return err
		}
		permission = c.CanSend(users[senderID], users[receiverID])
		return nil
	})
	if err != nil {
		return nil, err
	}
	return c, permission
}

func (s *messageSrv) SendWhisper(msg *ms.Message) (*ms.Message, error) {
	if msg.SenderUserID == msg.ReceiverUserID || msg.SenderUserID <= 0 || msg.ReceiverUserID <= 0 {
		return nil, dbr.ErrWhisperIdentity
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := lockIdentityChanges(tx, false); err != nil {
			return err
		}
		users, err := lockUsers(tx, msg.SenderUserID, msg.ReceiverUserID)
		if err != nil {
			return err
		}
		c, err := getConversation(tx, msg.SenderUserID, msg.ReceiverUserID, true)
		if err != nil {
			return err
		}
		sender, receiver := users[msg.SenderUserID], users[msg.ReceiverUserID]
		if err := c.CanSend(sender, receiver); err != nil {
			return err
		}
		c.Sent(sender)
		if err := tx.Save(c).Error; err != nil {
			return err
		}
		msg.Type = ms.MsgTypeWhisper
		return tx.Create(msg).Error
	})
	return msg, err
}

func (s *messageSrv) SetWhisperBlock(actorID, peerID int64, blocked bool) error {
	if actorID == peerID || peerID <= 0 {
		return dbr.ErrPermission
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := lockIdentityChanges(tx, false); err != nil {
			return err
		}
		users, err := lockUsers(tx, actorID, peerID)
		if err != nil {
			return err
		}
		actor, peer := users[actorID], users[peerID]
		if !actor.IsActive() || actor.AccountType != "member" || peer.AccountType != "member" {
			return dbr.ErrPermission
		}
		c, err := getConversation(tx, actorID, peerID, true)
		if err != nil {
			return err
		}
		if actorID == c.LowUserID {
			c.BlockedByLow = blocked
		} else {
			c.BlockedByHigh = blocked
		}
		if blocked {
			c.PendingSenderID = nil
		}
		return tx.Save(c).Error
	})
}
