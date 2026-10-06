package jinzhu

import (
	"errors"
	"math/rand/v2"
	"time"

	"github.com/BZYA-Community/WebsiteCore/internal/authz"
	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/jinzhu/dbr"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type reviewSrv struct{ db *gorm.DB }

func newReviewService(db *gorm.DB) core.ReviewService { return &reviewSrv{db: db} }

func validReviewKind(kind string) bool {
	switch kind {
	case ms.ReviewPost, ms.ReviewComment, ms.ReviewReply, ms.ReviewCourseQuestion,
		ms.ReviewCourseAnswer, ms.ReviewNickname, ms.ReviewAvatar:
		return true
	}
	return false
}

func reviewPolicyLock(tx *gorm.DB) error {
	var group dbr.IdentityGroup
	return tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("key = ?", "guest").First(&group).Error
}

func eligibleReviewers(tx *gorm.DB) ([]*ms.User, error) {
	var users []*ms.User
	if err := tx.Where("is_del = 0 AND status = ?", ms.UserStatusNormal).Find(&users).Error; err != nil {
		return nil, err
	}
	if err := dbr.LoadUserIdentities(tx, users...); err != nil {
		return nil, err
	}
	result := make([]*ms.User, 0, len(users))
	for _, user := range users {
		if user.HasPermission(authz.ContentReview) {
			result = append(result, user)
		}
	}
	return result, nil
}

func reviewEvent(tx *gorm.DB, task *ms.ReviewTask, event string, actorID, fromID, toID int64, reason string, now int64) error {
	return tx.Create(&ms.ReviewTaskEvent{TaskID: task.ID, Revision: task.Revision, Event: event,
		ActorID: actorID, FromAssigneeID: fromID, ToAssigneeID: toID, Reason: reason, CreatedOn: now}).Error
}

// enqueueReviewTx must be called after the whole content has been written in
// the same transaction. The caller locks reviewPolicyLock before content rows.
// A failed assignment/log write then rolls back the complete submission.
func enqueueReviewTx(tx *gorm.DB, kind string, targetID int64, deadline time.Duration) error {
	if !validReviewKind(kind) || targetID <= 0 || deadline <= 0 {
		return authz.ErrInvalid
	}
	subject, err := loadReviewSubject(tx, kind, targetID)
	if err != nil {
		return err
	}
	var task ms.ReviewTask
	err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("kind = ? AND target_id = ?", kind, targetID).First(&task).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	now := time.Now().Unix()
	if !subject.Pending {
		if task.ID > 0 && task.State == ms.ReviewPending {
			return cancelReviewTask(tx, &task, now)
		}
		return nil
	}
	if task.ID > 0 && task.State == ms.ReviewPending && task.Snapshot == subject.Snapshot {
		return nil
	}
	if task.ID == 0 {
		task = ms.ReviewTask{Kind: kind, TargetID: targetID, AuthorID: subject.AuthorID, Revision: 1,
			State: ms.ReviewPending, Snapshot: subject.Snapshot, CreatedOn: now, ModifiedOn: now}
		if err := tx.Create(&task).Error; err != nil {
			return err
		}
	} else {
		if err := reviewEvent(tx, &task, "superseded", subject.AuthorID, task.AssigneeID, 0, "", now); err != nil {
			return err
		}
		task.Revision++
		task.State, task.Snapshot = ms.ReviewPending, subject.Snapshot
		task.AssigneeID, task.PreviousAssigneeID, task.AssignedOn, task.DeadlineOn, task.CompletedOn = 0, 0, 0, 0, 0
		task.ModifiedOn = now
		if err := tx.Save(&task).Error; err != nil {
			return err
		}
	}
	if err := reviewEvent(tx, &task, "submitted", subject.AuthorID, 0, 0, "", now); err != nil {
		return err
	}
	reviewers, err := eligibleReviewers(tx)
	if err != nil {
		return err
	}
	return assignReviewTask(tx, &task, reviewers, deadline, now)
}

func assignReviewTask(tx *gorm.DB, task *ms.ReviewTask, reviewers []*ms.User, deadline time.Duration, now int64) error {
	ids := make([]int64, 0, len(reviewers))
	for _, user := range reviewers {
		if user.ID != task.AuthorID && user.ID != task.PreviousAssigneeID {
			ids = append(ids, user.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	task.AssigneeID = ids[rand.IntN(len(ids))]
	task.AssignedOn, task.DeadlineOn, task.ModifiedOn = now, now+int64(deadline/time.Second), now
	if err := tx.Save(task).Error; err != nil {
		return err
	}
	return reviewEvent(tx, task, "assigned", 0, task.PreviousAssigneeID, task.AssigneeID, "", now)
}

func cancelReviewTask(tx *gorm.DB, task *ms.ReviewTask, now int64) error {
	task.State, task.CompletedOn, task.ModifiedOn = ms.ReviewCancelled, now, now
	if err := tx.Save(task).Error; err != nil {
		return err
	}
	return reviewEvent(tx, task, "cancelled", 0, task.AssigneeID, 0, "Content is no longer pending", now)
}

func cancelTargetReviewsTx(tx *gorm.DB, kind string, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	var tasks []*ms.ReviewTask
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("kind = ? AND target_id IN ? AND state = ?", kind, ids, ms.ReviewPending).Find(&tasks).Error; err != nil {
		return err
	}
	for _, task := range tasks {
		if err := cancelReviewTask(tx, task, time.Now().Unix()); err != nil {
			return err
		}
	}
	return nil
}

// ReconcileReviewTasks is safe with multiple server instances. The existing
// policy row lock serializes assignment with membership/status changes.
// ponytail: a single bounded-frequency sweep; shard only if measured queue size requires it.
func (s *reviewSrv) ReconcileReviewTasks(deadline time.Duration) error {
	if deadline < time.Second {
		return authz.ErrInvalid
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := reviewPolicyLock(tx); err != nil {
			return err
		}
		reviewers, err := eligibleReviewers(tx)
		if err != nil {
			return err
		}
		eligible := make(map[int64]bool, len(reviewers))
		for _, user := range reviewers {
			eligible[user.ID] = true
		}
		var tasks []*ms.ReviewTask
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("state = ?", ms.ReviewPending).Order("id").Find(&tasks).Error; err != nil {
			return err
		}
		now := time.Now().Unix()
		for _, task := range tasks {
			subject, err := loadReviewSubject(tx, task.Kind, task.TargetID)
			if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && !subject.Pending) {
				if err := cancelReviewTask(tx, task, now); err != nil {
					return err
				}
				continue
			}
			if err != nil {
				return err
			}
			if subject.Snapshot != task.Snapshot {
				// A replacement must get a new revision through the submission transaction.
				if err := enqueueReviewTx(tx, task.Kind, task.TargetID, deadline); err != nil {
					return err
				}
				continue
			}
			if task.AssigneeID != 0 {
				event := ""
				switch {
				case !eligible[task.AssigneeID] || task.AssigneeID == task.AuthorID:
					event = "ineligible"
				case task.DeadlineOn <= now:
					event = "timeout"
				}
				if event == "" {
					continue
				}
				if err := reviewEvent(tx, task, event, 0, task.AssigneeID, 0, "", now); err != nil {
					return err
				}
				task.PreviousAssigneeID = task.AssigneeID
				task.AssigneeID, task.AssignedOn, task.DeadlineOn, task.ModifiedOn = 0, 0, 0, now
				if err := tx.Save(task).Error; err != nil {
					return err
				}
			}
			if err := assignReviewTask(tx, task, reviewers, deadline, now); err != nil {
				return err
			}
		}
		return nil
	})
}

func freshReviewActor(db *gorm.DB, actor *ms.User) (*ms.User, error) {
	if actor == nil || actor.Model == nil || actor.ID <= 0 {
		return nil, authz.ErrDenied
	}
	user, err := (&ms.User{Model: &ms.Model{ID: actor.ID}}).Get(db)
	if err != nil {
		return nil, err
	}
	if !user.HasPermission(authz.ContentReview) && !user.HasPermission(authz.AuditViewAll) {
		return nil, authz.ErrDenied
	}
	return user, nil
}

func reviewTaskScope(db *gorm.DB, actor *ms.User) *gorm.DB {
	if actor.HasPermission(authz.AuditViewAll) {
		return db
	}
	return db.Where("assignee_id = ?", actor.ID)
}

func (s *reviewSrv) ListReviewTasks(actor *ms.User, kind, state string, offset, limit int) ([]*ms.ReviewTask, int64, error) {
	actor, err := freshReviewActor(s.db, actor)
	if err != nil {
		return nil, 0, err
	}
	if (kind != "" && !validReviewKind(kind)) || (state != "" && state != ms.ReviewPending && state != ms.ReviewCompleted && state != ms.ReviewCancelled) {
		return nil, 0, authz.ErrInvalid
	}
	db := reviewTaskScope(s.db.Model(&ms.ReviewTask{}), actor)
	if kind != "" {
		db = db.Where("kind = ?", kind)
	}
	if state != "" {
		db = db.Where("state = ?", state)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if offset < 0 || limit < 1 || limit > 100 {
		return nil, 0, authz.ErrInvalid
	}
	tasks := []*ms.ReviewTask{}
	err = db.Order("id DESC").Offset(offset).Limit(limit).Find(&tasks).Error
	return tasks, total, err
}

func (s *reviewSrv) ReviewTaskForTarget(actor *ms.User, kind string, targetID int64) (*ms.ReviewTask, error) {
	actor, err := freshReviewActor(s.db, actor)
	if err != nil {
		return nil, err
	}
	var task ms.ReviewTask
	err = reviewTaskScope(s.db, actor).Where("kind = ? AND target_id = ?", kind, targetID).First(&task).Error
	return &task, err
}

func (s *reviewSrv) ListReviewTaskEvents(actor *ms.User, taskID int64) ([]*ms.ReviewTaskEvent, error) {
	actor, err := freshReviewActor(s.db, actor)
	if err != nil {
		return nil, err
	}
	var task ms.ReviewTask
	if err := reviewTaskScope(s.db, actor).Where("id = ?", taskID).First(&task).Error; err != nil {
		return nil, err
	}
	events := []*ms.ReviewTaskEvent{}
	err = s.db.Where("task_id = ?", taskID).Order("id ASC").Find(&events).Error
	return events, err
}

func (s *reviewSrv) ReviewerTimeoutStatistics(actor *ms.User) ([]*ms.ReviewerStatistics, error) {
	actor, err := freshReviewActor(s.db, actor)
	if err != nil {
		return nil, err
	}
	db := s.db.Model(&ms.ReviewTaskEvent{}).Where("event = ?", "timeout")
	if !actor.HasPermission(authz.AuditViewAll) {
		db = db.Where("from_assignee_id = ?", actor.ID)
	}
	stats := []*ms.ReviewerStatistics{}
	err = db.Select("from_assignee_id AS user_id, COUNT(*) AS timeout_count").Group("from_assignee_id").Order("from_assignee_id").Scan(&stats).Error
	return stats, err
}
