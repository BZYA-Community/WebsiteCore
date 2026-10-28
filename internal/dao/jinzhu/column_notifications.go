package jinzhu

import (
	"context"

	"github.com/BZYA-Community/WebsiteCore/internal/conf"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/cache"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type columnNoticeKey struct{}

// Commit notifications with their content, then expire only the affected unread
// caches. A Redis failure cannot undo a committed moderation decision.
func columnTransaction(db *gorm.DB, fn func(*gorm.DB) error) error {
	recipients := map[int64]bool{}
	err := db.Transaction(func(tx *gorm.DB) error {
		return fn(tx.WithContext(context.WithValue(tx.Statement.Context, columnNoticeKey{}, recipients)))
	})
	if err != nil {
		return err
	}
	if conf.CacheSetting != nil && len(recipients) > 0 {
		wc := cache.NewWebCache()
		for id := range recipients {
			if err := wc.DelUnreadMsgCountResp(id); err != nil {
				logrus.WithError(err).Warn("column notification unread-cache refresh failed")
			}
		}
	}
	return nil
}
