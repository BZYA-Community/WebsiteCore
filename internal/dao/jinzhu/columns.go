package jinzhu

import (
	"encoding/json"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/BZYA-Community/WebsiteCore/internal/application/columnindex"
	"github.com/BZYA-Community/WebsiteCore/internal/authz"
	"github.com/BZYA-Community/WebsiteCore/internal/conf"
	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type columnSrv struct {
	db    *gorm.DB
	index *columnindex.Index
}

func publicColumns(db *gorm.DB) *gorm.DB {
	return db.Where("published_on > 0 AND blocked = ? AND is_del = 0", false)
}

func loadColumn(db *gorm.DB, id int64) (*ms.Column, error) {
	if id <= 0 {
		return nil, authz.ErrInvalid
	}
	row := &ms.Column{}
	err := db.Where("id = ? AND is_del = 0", id).First(row).Error
	return row, err
}

func columnManager(actor *ms.User, row *ms.Column) bool {
	return actor != nil && (actor.ID == row.UserID || actor.HasPermission(authz.ContentManage))
}

func columnPublic(row *ms.Column) bool { return row.PublishedOn > 0 && !row.Blocked && row.IsDel == 0 }

func columnUser(db *gorm.DB, id int64) *ms.UserFormated {
	user, err := (&ms.User{Model: &ms.Model{ID: id}}).Get(db)
	if err != nil {
		return ms.GhostUserFormated
	}
	return user.Format()
}

func lockColumnActor(tx *gorm.DB, actor *ms.User) (*ms.User, error) {
	if actor == nil || actor.Model == nil || actor.ID <= 0 {
		return nil, authz.ErrDenied
	}
	if err := reviewPolicyLock(tx); err != nil {
		return nil, err
	}
	fresh, err := (&ms.User{Model: &ms.Model{ID: actor.ID}}).Get(tx)
	if err != nil {
		return nil, err
	}
	if fresh.Status != ms.UserStatusNormal {
		return nil, authz.ErrDenied
	}
	return fresh, nil
}

func (s *columnSrv) ListColumns(actor *ms.User, q core.ColumnQuery) ([]*ms.Column, int64, error) {
	if (q.Kind != ms.ColumnArticle && q.Kind != ms.ColumnCollection) || q.Offset < 0 || q.Limit < 1 || q.Limit > 100 {
		return nil, 0, authz.ErrInvalid
	}
	db := s.db.Model(&ms.Column{}).Where("kind = ? AND is_del = 0", q.Kind)
	if q.Returned {
		if actor == nil || actor.ID <= 0 || !q.Mine || q.Manage || q.Kind != ms.ColumnArticle {
			return nil, 0, authz.ErrDenied
		}
		db = s.db.Unscoped().Model(&ms.Column{}).Where("kind=? AND is_del=1 AND admin_deleted_by>0 AND user_id=?", ms.ColumnArticle, actor.ID)
	} else if q.Mine || q.Manage {
		if actor == nil || actor.ID <= 0 || (q.Manage && !actor.HasPermission(authz.ContentManage)) {
			return nil, 0, authz.ErrDenied
		}
		if !q.Manage {
			db = db.Where("user_id = ?", actor.ID)
		}
	} else {
		db = publicColumns(db)
		if q.Kind == ms.ColumnCollection {
			entries, cols := s.db.NamingStrategy.TableName("column_entry"), s.db.NamingStrategy.TableName("column")
			db = db.Where("EXISTS (SELECT 1 FROM " + entries + " e JOIN " + cols + " a ON a.id=e.article_id WHERE e.collection_id=" + cols + ".id AND a.is_del=0 AND NOT a.blocked AND a.published_on>0)")
		}
	}
	if q.AuthorID > 0 {
		db = db.Where("user_id=?", q.AuthorID)
	}
	if q.Official {
		db = db.Where("official = ?", true)
	}
	if keyword := strings.TrimSpace(q.Keyword); keyword != "" {
		if q.Returned {
			matches := columnKeyword(s.db.Model(&ms.ColumnSubmission{}).Select("id"), keyword, ms.ColumnArticle)
			db = db.Where("submission_id IN (?)", matches)
		} else if !q.Mine && !q.Manage && q.Kind == ms.ColumnArticle {
			if ids, ok := s.index.Search(keyword); ok {
				db = db.Where("id IN ?", ids)
			} else {
				db = columnKeyword(db, keyword, q.Kind)
			}
		} else {
			db = columnKeyword(db, keyword, q.Kind)
		}
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	rows := []*ms.Column{}
	order := "published_on DESC, id DESC"
	if q.Mine || q.Manage {
		order = "id DESC"
	}
	if q.Returned {
		order = "admin_deleted_on DESC, id DESC"
	}
	if err := db.Order(order).Offset(q.Offset).Limit(q.Limit).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	for _, row := range rows {
		row.User = columnUser(s.db, row.UserID)
		if q.Mine || q.Manage {
			var submission ms.ColumnSubmission
			if row.SubmissionID > 0 && s.db.First(&submission, row.SubmissionID).Error == nil {
				row.Submission = &submission
				if q.Returned {
					row.Title, row.Summary = submission.Title, submission.Summary
				}
			}
		}
		row.Body, row.Attachments = "", nil
	}
	return rows, total, nil
}

func columnKeyword(db *gorm.DB, keyword, kind string) *gorm.DB {
	keyword = strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(keyword)
	pattern := "%" + keyword + "%"
	if kind == ms.ColumnCollection {
		return db.Where("(title ILIKE ? OR summary ILIKE ?)", pattern, pattern)
	}
	return db.Where("(title ILIKE ? OR summary ILIKE ? OR body ILIKE ?)", pattern, pattern, pattern)
}

func (s *columnSrv) GetColumn(actor *ms.User, id int64, submission bool) (*ms.Column, error) {
	row, err := loadColumn(s.db, id)
	if err != nil {
		return nil, err
	}
	if submission {
		if !columnManager(actor, row) {
			return nil, authz.ErrDenied
		}
		if row.Kind == ms.ColumnArticle && actor.ID != row.UserID {
			return nil, authz.ErrDenied
		}
		if row.SubmissionID > 0 {
			row.Submission = &ms.ColumnSubmission{}
			if err := s.db.First(row.Submission, row.SubmissionID).Error; err != nil {
				return nil, err
			}
		}
	} else if !columnPublic(row) {
		return nil, gorm.ErrRecordNotFound
	}
	row.User = columnUser(s.db, row.UserID)
	return row, nil
}

func normalizeColumnSubmission(kind string, r *ms.ColumnSubmission) error {
	if r == nil {
		return authz.ErrInvalid
	}
	r.Title, r.Summary = strings.TrimSpace(r.Title), strings.TrimSpace(r.Summary)
	if r.Title == "" || utf8.RuneCountInString(r.Title) > 200 || utf8.RuneCountInString(r.Summary) > 2000 || utf8.RuneCountInString(r.Body) > 50000 {
		return authz.ErrInvalid
	}
	if kind == ms.ColumnArticle {
		if strings.TrimSpace(r.Body) == "" || r.Cover != "" {
			return authz.ErrInvalid
		}
		if r.Summary == "" {
			runes := []rune(r.Body)
			if len(runes) > 200 {
				runes = runes[:200]
			}
			r.Summary = string(runes)
		}
	} else if kind == ms.ColumnCollection {
		if r.Body != "" {
			return authz.ErrInvalid
		}
	} else {
		return authz.ErrInvalid
	}
	if r.Attachments == nil {
		r.Attachments = []*ms.ColumnContent{}
	}
	return nil
}

func (s *columnSrv) SubmitColumn(actor *ms.User, id, expected int64, kind string, input *ms.ColumnSubmission) (*ms.Column, error) {
	return s.submitColumn(actor, id, expected, 0, kind, input)
}

func (s *columnSrv) RecreateColumn(actor *ms.User, sourceID int64, input *ms.ColumnSubmission) (*ms.Column, error) {
	if sourceID <= 0 {
		return nil, authz.ErrInvalid
	}
	return s.submitColumn(actor, 0, 0, sourceID, ms.ColumnArticle, input)
}

func (s *columnSrv) submitColumn(actor *ms.User, id, expected, sourceID int64, kind string, input *ms.ColumnSubmission) (*ms.Column, error) {
	if err := normalizeColumnSubmission(kind, input); err != nil {
		return nil, err
	}
	var row *ms.Column
	err := columnTransaction(s.db, func(tx *gorm.DB) error {
		fresh, err := lockIdentityPolicy(tx, actor, authz.ColumnPublish)
		if err != nil {
			return err
		}
		if id == 0 {
			row = &ms.Column{Model: &ms.Model{}, UserID: fresh.ID, Kind: kind, Curated: kind == ms.ColumnCollection && fresh.HasPermission(authz.ContentManage), Attachments: []*ms.ColumnContent{}}
			if sourceID > 0 {
				if _, err := returnedColumn(tx, fresh, sourceID); err != nil {
					return err
				}
				row.RecreatedFromID = sourceID
			}
			if err := tx.Create(row).Error; err != nil {
				return err
			}
		} else {
			row, err = loadColumn(tx.Clauses(clause.Locking{Strength: "UPDATE"}), id)
			if err != nil {
				return err
			}
			if row.Kind != kind || row.Blocked || (row.Curated && !fresh.HasPermission(authz.ContentManage)) || (row.UserID != fresh.ID && (kind != ms.ColumnCollection || !fresh.HasPermission(authz.ContentManage))) {
				return authz.ErrDenied
			}
			if row.SubmissionID != expected {
				return core.ErrReviewStale
			}
			if row.SubmissionID > 0 {
				var old ms.ColumnSubmission
				if err := tx.First(&old, row.SubmissionID).Error; err != nil {
					return err
				}
				if old.AuditStatus == ms.PostAuditPending {
					return core.ErrReviewStale
				}
			}
		}
		media := append([]*ms.ColumnContent{}, input.Attachments...)
		if input.Cover != "" {
			media = append(media, &ms.ColumnContent{Content: input.Cover, Type: ms.ContentTypeImage})
		}
		if err := checkColumnMediaTx(tx, fresh, row, media); err != nil {
			return err
		}
		r := *input
		r.Model = &ms.Model{}
		r.ColumnID = row.ID
		r.UserID = fresh.ID
		r.Reason = ""
		r.AuditStatus = ms.PostAuditApproved
		if reviewRequired(fresh) || (row.RecreatedFromID > 0 && row.PublicSubmissionID == 0) {
			r.AuditStatus = ms.PostAuditPending
		}
		if err := tx.Create(&r).Error; err != nil {
			return err
		}
		row.SubmissionID = r.ID
		if err := tx.Model(&ms.Column{}).Where("id=?", row.ID).Update("submission_id", r.ID).Error; err != nil {
			return err
		}
		if r.AuditStatus == ms.PostAuditApproved {
			if err := publishColumnTx(tx, row, &r); err != nil {
				return err
			}
		}
		if err := enqueueReviewTx(tx, ms.ReviewColumn, r.ID, conf.ReviewDeadline()); err != nil {
			return err
		}
		row.Submission = &r
		return nil
	})
	if err == nil {
		syncColumnIndex(s.index, row.ID)
	}
	return row, err
}

func publishColumnTx(tx *gorm.DB, row *ms.Column, r *ms.ColumnSubmission) error {
	now := time.Now().Unix()
	published := row.PublishedOn
	if published == 0 {
		published = now
	}
	attachments, err := json.Marshal(r.Attachments)
	if err != nil {
		return err
	}
	changes := map[string]any{"title": r.Title, "summary": r.Summary, "body": r.Body, "cover": r.Cover, "attachments": string(attachments), "published_on": published, "updated_on": now, "public_submission_id": r.ID}
	if err := tx.Model(&ms.Column{}).Where("id=? AND is_del=0 AND NOT blocked", row.ID).Updates(changes).Error; err != nil {
		return err
	}
	row.PublicSubmissionID = r.ID
	row.Title, row.Summary, row.Body, row.Cover, row.Attachments, row.PublishedOn, row.UpdatedOn = r.Title, r.Summary, r.Body, r.Cover, r.Attachments, published, now
	return nil
}

func (s *columnSrv) ColumnAction(actor *ms.User, id int64, action string) error {
	return s.columnAction(actor, id, action, "")
}

func (s *columnSrv) DeleteColumn(actor *ms.User, id int64, reason string) error {
	return s.columnAction(actor, id, "delete", reason)
}

func (s *columnSrv) columnAction(actor *ms.User, id int64, action, reason string) error {
	reason = strings.TrimSpace(reason)
	if utf8.RuneCountInString(reason) > 255 {
		return authz.ErrInvalid
	}
	err := columnTransaction(s.db, func(tx *gorm.DB) error {
		fresh, err := lockColumnActor(tx, actor)
		if err != nil {
			return err
		}
		row, err := loadColumn(tx.Clauses(clause.Locking{Strength: "UPDATE"}), id)
		if err != nil {
			return err
		}
		if err := columnActionTx(tx, fresh, row, action, reason); err != nil {
			return err
		}
		return tx.Create(&ms.AuditLog{Model: &ms.Model{}, ColumnID: id, OperatorID: fresh.ID, Action: "column_" + action, Reason: reason}).Error
	})
	if err == nil {
		syncColumnIndex(s.index, id)
	}
	return err
}

func columnActionTx(tx *gorm.DB, fresh *ms.User, row *ms.Column, action, reason string) error {
	id := row.ID
	manager := fresh.HasPermission(authz.ContentManage)
	switch action {
	case "withdraw":
		if row.Blocked || (row.Curated && !manager) || !fresh.HasPermission(authz.ColumnPublish) || (row.UserID != fresh.ID && (row.Kind != ms.ColumnCollection || !manager)) {
			return authz.ErrDenied
		}
		return withdrawColumnTx(tx, row)
	case "delete":
		if (row.Curated && !manager) || (row.UserID != fresh.ID && !manager) {
			return authz.ErrDenied
		}
		if row.Kind == ms.ColumnArticle && manager && fresh.ID != row.UserID {
			if err := returnDeletedColumnTx(tx, fresh, row, reason); err != nil {
				return err
			}
		}
		if err := withdrawColumnTx(tx, row); err != nil {
			return err
		}
		if err := cancelColumnCommentsTx(tx, id); err != nil {
			return err
		}
		if err := tx.Where("collection_id=? OR article_id=?", id, id).Delete(&ms.ColumnEntry{}).Error; err != nil {
			return err
		}
		return tx.Delete(row).Error
	case "takedown", "restore":
		if !manager {
			return authz.ErrDenied
		}
		if action == "takedown" {
			if err := withdrawColumnTx(tx, row); err != nil {
				return err
			}
			if err := cancelColumnCommentsTx(tx, id); err != nil {
				return err
			}
		}
		return tx.Model(row).Update("blocked", action == "takedown").Error
	case "lock", "unlock":
		if row.Kind != ms.ColumnArticle || (!manager && (row.UserID != fresh.ID || !fresh.HasPermission(authz.ColumnPublish))) {
			return authz.ErrDenied
		}
		return tx.Model(row).Update("locked", action == "lock").Error
	case "invite", "uninvite":
		if !manager || row.Kind != ms.ColumnArticle {
			return authz.ErrDenied
		}
		return tx.Model(row).Update("official", action == "invite").Error
	default:
		return authz.ErrInvalid
	}
}

func withdrawColumnTx(tx *gorm.DB, row *ms.Column) error {
	if row.SubmissionID == 0 {
		return nil
	}
	if err := tx.Model(&ms.ColumnSubmission{}).Where("id=? AND audit_status=?", row.SubmissionID, ms.PostAuditPending).Updates(map[string]any{"audit_status": ms.ColumnWithdrawn}).Error; err != nil {
		return err
	}
	return cancelTargetReviewsTx(tx, ms.ReviewColumn, []int64{row.SubmissionID})
}

func cancelColumnCommentsTx(tx *gorm.DB, id int64) error {
	var ids []int64
	if err := tx.Model(&ms.ColumnComment{}).Where("column_id=? AND audit_status=?", id, ms.PostAuditPending).Pluck("id", &ids).Error; err != nil {
		return err
	}
	if err := tx.Model(&ms.ColumnComment{}).Where("id IN ?", ids).Update("audit_status", ms.ColumnWithdrawn).Error; err != nil {
		return err
	}
	return cancelTargetReviewsTx(tx, ms.ReviewColumnComment, ids)
}

func (s *columnSrv) ColumnEntries(actor *ms.User, id int64, editing bool) ([]*ms.Column, error) {
	collection, err := loadColumn(s.db, id)
	if err != nil {
		return nil, err
	}
	if collection.Kind != ms.ColumnCollection {
		return nil, authz.ErrInvalid
	}
	if editing {
		if !columnManager(actor, collection) {
			return nil, authz.ErrDenied
		}
	} else if !columnPublic(collection) {
		return nil, gorm.ErrRecordNotFound
	}
	var entries []ms.ColumnEntry
	if err := s.db.Where("collection_id=?", id).Order("position ASC").Find(&entries).Error; err != nil {
		return nil, err
	}
	rows := []*ms.Column{}
	for _, entry := range entries {
		row, err := loadColumn(s.db, entry.ArticleID)
		if err != nil || !columnPublic(row) {
			continue
		}
		row.Body, row.Attachments = "", nil
		row.User = columnUser(s.db, row.UserID)
		rows = append(rows, row)
	}
	return rows, nil
}

func (s *columnSrv) SetColumnEntries(actor *ms.User, id int64, articles []int64) error {
	if len(articles) > 1000 {
		return authz.ErrInvalid
	}
	return columnTransaction(s.db, func(tx *gorm.DB) error {
		fresh, err := lockColumnActor(tx, actor)
		if err != nil {
			return err
		}
		collection, err := loadColumn(tx, id)
		if err != nil {
			return err
		}
		manager := fresh.HasPermission(authz.ContentManage)
		if collection.Kind != ms.ColumnCollection || collection.Blocked || (collection.Curated && !manager) || !columnManager(fresh, collection) || (!manager && !fresh.HasPermission(authz.ColumnPublish)) {
			return authz.ErrDenied
		}
		var old []ms.ColumnEntry
		if err := tx.Where("collection_id=?", id).Find(&old).Error; err != nil {
			return err
		}
		previous := map[int64]bool{}
		for _, e := range old {
			previous[e.ArticleID] = true
		}
		seen := map[int64]bool{}
		rows := []ms.ColumnEntry{}
		for i, articleID := range articles {
			if seen[articleID] {
				return authz.ErrInvalid
			}
			seen[articleID] = true
			article, err := loadColumn(tx, articleID)
			if err != nil {
				return err
			}
			if article.Kind != ms.ColumnArticle || !columnPublic(article) || (!collection.Curated && article.UserID != collection.UserID) || (!manager && collection.Curated) {
				return authz.ErrDenied
			}
			rows = append(rows, ms.ColumnEntry{CollectionID: id, ArticleID: articleID, Position: i})
			if article.UserID != fresh.ID && !previous[articleID] {
				if err := columnMessage(tx, fresh.ID, article.UserID, collection.ID, 0, "你的专栏文章已被收录到合集："+collection.Title); err != nil {
					return err
				}
			}
		}
		if err := tx.Where("collection_id=?", id).Delete(&ms.ColumnEntry{}).Error; err != nil {
			return err
		}
		if len(rows) > 0 {
			return tx.Create(&rows).Error
		}
		return nil
	})
}

func columnMessage(tx *gorm.DB, sender, receiver, columnID, commentID int64, brief string) error {
	return storeColumnMessage(tx, &ms.Message{Model: &ms.Model{}, SenderUserID: sender, ReceiverUserID: receiver, Type: ms.MsgTypeSystem, ColumnID: columnID, CommentID: commentID, Brief: brief})
}

func storeColumnMessage(tx *gorm.DB, message *ms.Message) error {
	if message.ReceiverUserID == 0 || message.ReceiverUserID == message.SenderUserID {
		return nil
	}
	if runes := []rune(message.Brief); len(runes) > 255 {
		message.Brief = string(runes[:254]) + "…"
	}
	if err := tx.Create(message).Error; err != nil {
		return err
	}
	if recipients, ok := tx.Statement.Context.Value(columnNoticeKey{}).(map[int64]bool); ok {
		recipients[message.ReceiverUserID] = true
	}
	return nil
}

func (s *columnSrv) ColumnAttachment(actor *ms.User, id, submissionID int64, index int) (string, error) {
	row, err := loadColumn(s.db, id)
	if err != nil {
		return "", err
	}
	contents := row.Attachments
	if submissionID > 0 {
		var submission ms.ColumnSubmission
		if err := s.db.Where("id=? AND column_id=?", submissionID, id).First(&submission).Error; err != nil {
			return "", err
		}
		if !columnManager(actor, row) {
			if _, err := (&reviewSrv{db: s.db}).ReviewTaskForTarget(actor, ms.ReviewColumn, submissionID); err != nil {
				return "", authz.ErrDenied
			}
		}
		contents = submission.Attachments
	} else if !columnPublic(row) {
		return "", authz.ErrDenied
	}
	if index < 0 || index >= len(contents) || contents[index].Type != ms.ContentTypeAttachment {
		return "", authz.ErrInvalid
	}
	return contents[index].Content, nil
}
