package dbr

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

type OperationLog struct {
	ID        int64  `gorm:"primaryKey" json:"id"`
	ActorID   int64  `json:"actor_id"`
	Action    string `gorm:"size:32" json:"action"`
	Entity    string `gorm:"size:32" json:"entity"`
	EntityID  int64  `json:"entity_id"`
	Before    string `gorm:"column:before_json;type:text" json:"before"`
	After     string `gorm:"column:after_json;type:text" json:"after"`
	CreatedOn int64  `json:"created_on"`
}

// AppendOperationLog participates in the caller's transaction. Never pass
// passwords, verification codes, private resource keys, or other secrets.
func AppendOperationLog(db *gorm.DB, actorID int64, action, entity string, entityID int64, before, after any) error {
	oldJSON, err := json.Marshal(before)
	if err != nil {
		return err
	}
	newJSON, err := json.Marshal(after)
	if err != nil {
		return err
	}
	return db.Create(&OperationLog{ActorID: actorID, Action: action, Entity: entity, EntityID: entityID,
		Before: string(oldJSON), After: string(newJSON), CreatedOn: time.Now().Unix()}).Error
}
