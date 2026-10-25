package jinzhu

import (
	"fmt"
	"strings"
	"time"

	"github.com/BZYA-Community/WebsiteCore/internal/authz"
	"github.com/BZYA-Community/WebsiteCore/internal/conf"
	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/jinzhu/dbr"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var _ core.ProjectService = (*projectSrv)(nil)

type projectSrv struct{ db *gorm.DB }

func newProjectService(db *gorm.DB) core.ProjectService { return &projectSrv{db: db} }

func projectTablePrefix() string {
	if conf.DatabaseSetting == nil {
		return "p_"
	}
	return conf.DatabaseSetting.TablePrefix
}

func projectVisibleScope(db *gorm.DB, viewer *ms.User) *gorm.DB {
	prefix := projectTablePrefix()
	db = db.Table(prefix+"project AS project").
		Joins("JOIN "+prefix+"post AS post ON post.id = project.post_id AND post.is_del = 0").
		Where("project.is_del = ?", 0)
	if viewer == nil {
		return db.Where("post.audit_status = ? AND post.visibility = ?", ms.PostAuditApproved, ms.PostVisitPublic)
	}
	if !viewer.HasPermission(authz.PostView) {
		return db.Where("1 = 0")
	}

	// Keep project discovery on the same policy as content.Views.CanViewTweet.
	// A project is a separate document, but its linked post remains the source
	// of truth for publication state and visibility.
	if viewer.HasPermission(authz.AuditViewAll) {
		return db.Where("post.user_id = ? OR post.audit_status <> ? OR post.visibility = ? OR ? OR (post.visibility = ? AND EXISTS (SELECT 1 FROM "+prefix+"following f WHERE f.user_id = ? AND f.follow_id = post.user_id AND f.is_del = 0))",
			viewer.ID, ms.PostAuditApproved, ms.PostVisitPublic, viewer.HasPermission(authz.ContentViewPrivate), ms.PostVisitFollowing, viewer.ID)
	}
	pending := "FALSE"
	args := []any{viewer.ID, ms.PostAuditApproved}
	if viewer.HasPermission(authz.ContentReview) {
		pending = "EXISTS (SELECT 1 FROM " + prefix + "review_task rt WHERE rt.kind = ? AND rt.target_id = post.id AND rt.assignee_id = ?)"
		args = append(args, ms.ReviewPost, viewer.ID)
	}
	args = append(args, ms.PostAuditApproved, ms.PostVisitPublic, viewer.HasPermission(authz.ContentViewPrivate), ms.PostVisitFollowing, viewer.ID)
	return db.Where("post.user_id = ? OR (post.audit_status <> ? AND "+pending+") OR (post.audit_status = ? AND (post.visibility = ? OR ? OR (post.visibility = ? AND EXISTS (SELECT 1 FROM "+prefix+"following f WHERE f.user_id = ? AND f.follow_id = post.user_id AND f.is_del = 0))))", args...)
}

func projectAuditScope(db *gorm.DB, actor *ms.User) *gorm.DB {
	tasks := reviewTaskScope(db.Session(&gorm.Session{NewDB: true}).Model(&ms.ReviewTask{}), actor).
		Select("target_id").Where("kind = ?", ms.ReviewPost)
	return db.Table(projectTablePrefix()+"project AS project").
		Where("project.is_del = ?", 0).
		Where("project.post_id IN (?)", tasks)
}

func projectFollowedScope(db *gorm.DB, viewer *ms.User, followed bool) (*gorm.DB, string) {
	if !followed {
		return db, "project.id DESC"
	}
	if viewer == nil {
		return db.Where("1 = 0"), "project.id DESC"
	}
	prefix := projectTablePrefix()
	return db.Joins("JOIN "+prefix+"project_follow AS project_follow ON project_follow.project_id = project.id").
		Where("project_follow.user_id = ?", viewer.ID), "project_follow.created_on DESC, project.id DESC"
}

func (s *projectSrv) ListProjects(viewer *ms.User, keyword, tag string, followed bool, offset, limit int) ([]*ms.ProjectFormated, int64, error) {
	db := projectVisibleScope(s.db, viewer)
	db, order := projectFollowedScope(db, viewer, followed)
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		db = db.Where("LOWER(project.title) LIKE LOWER(?)", "%"+keyword+"%")
	}
	if tag = strings.TrimSpace(tag); tag != "" {
		db = db.Where("EXISTS (SELECT 1 FROM "+projectTablePrefix()+"project_tag pt WHERE pt.project_id = project.id AND pt.is_del = 0 AND pt.name = ?)", tag)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var projects []*dbr.Project
	if err := db.Select("project.*").Order(order).Offset(offset).Limit(limit).Find(&projects).Error; err != nil {
		return nil, 0, err
	}
	viewerID := int64(0)
	if viewer != nil {
		viewerID = viewer.ID
	}
	items, err := s.formatProjects(projects, viewerID)
	return items, total, err
}

func (s *projectSrv) GetProject(viewer *ms.User, id int64) (*ms.ProjectFormated, error) {
	var project dbr.Project
	if err := projectVisibleScope(s.db, viewer).Select("project.*").Where("project.id = ?", id).First(&project).Error; err != nil {
		return nil, err
	}
	viewerID := int64(0)
	if viewer != nil {
		viewerID = viewer.ID
	}
	items, err := s.formatProjects([]*dbr.Project{&project}, viewerID)
	if err != nil || len(items) == 0 {
		return nil, err
	}
	return items[0], nil
}

func (s *projectSrv) GetAuditProjects(actor *ms.User, postIDs []int64) (map[int64]*ms.ProjectFormated, error) {
	result := make(map[int64]*ms.ProjectFormated)
	if len(postIDs) == 0 {
		return result, nil
	}
	actor, err := freshReviewActor(s.db, actor)
	if err != nil {
		return nil, err
	}
	var projects []*dbr.Project
	if err := projectAuditScope(s.db, actor).Where("project.post_id IN ?", postIDs).Find(&projects).Error; err != nil {
		return nil, err
	}
	items, err := s.formatProjects(projects, actor.ID)
	if err != nil {
		return nil, err
	}
	for _, project := range items {
		result[project.PostID] = project
	}
	return result, nil
}

func (s *projectSrv) formatProjects(projects []*dbr.Project, viewerID int64) ([]*ms.ProjectFormated, error) {
	if len(projects) == 0 {
		return []*ms.ProjectFormated{}, nil
	}
	ids := make([]int64, 0, len(projects))
	creatorIDs := make([]int64, 0, len(projects))
	for _, project := range projects {
		ids = append(ids, project.ID)
		creatorIDs = append(creatorIDs, project.CreatorID)
	}
	var images []*dbr.ProjectImage
	if err := s.db.Where("project_id IN ? AND is_del = ?", ids, 0).Order("sort, id").Find(&images).Error; err != nil {
		return nil, err
	}
	var tags []*dbr.ProjectTag
	if err := s.db.Where("project_id IN ? AND is_del = ?", ids, 0).Order("name").Find(&tags).Error; err != nil {
		return nil, err
	}
	var members []*dbr.ProjectMember
	if err := s.db.Where("project_id IN ? AND is_del = ?", ids, 0).Order("id").Find(&members).Error; err != nil {
		return nil, err
	}
	userIDs := append([]int64{}, creatorIDs...)
	for _, member := range members {
		userIDs = append(userIDs, member.UserID)
	}
	users, err := getUsersByIDs(s.db, userIDs)
	if err != nil {
		return nil, err
	}
	userMap := make(map[int64]*dbr.UserFormated, len(users))
	for _, user := range users {
		userMap[user.ID] = user.Format()
	}
	followed := map[int64]bool{}
	if viewerID > 0 {
		var follows []*dbr.ProjectFollow
		if err := s.db.Where("project_id IN ? AND user_id = ?", ids, viewerID).Find(&follows).Error; err != nil {
			return nil, err
		}
		for _, follow := range follows {
			followed[follow.ProjectID] = true
		}
	}
	result := make([]*ms.ProjectFormated, 0, len(projects))
	for _, project := range projects {
		item := &ms.ProjectFormated{
			ID: project.ID, CreatorID: project.CreatorID, Creator: userMap[project.CreatorID], PostID: project.PostID,
			Title: project.Title, Intro: project.Intro, URL: project.URL, Images: []string{}, Tags: []string{},
			Members: []*ms.ProjectMemberFormated{}, FollowCount: project.FollowCount, Followed: followed[project.ID], CreatedOn: project.CreatedOn,
		}
		if item.Creator == nil {
			item.Creator = ms.GhostUserFormated
		}
		for _, image := range images {
			if image.ProjectID == project.ID {
				item.Images = append(item.Images, image.URL)
			}
		}
		for _, tag := range tags {
			if tag.ProjectID == project.ID {
				item.Tags = append(item.Tags, tag.Name)
			}
		}
		for _, member := range members {
			if member.ProjectID == project.ID {
				user := userMap[member.UserID]
				if user == nil {
					user = ms.GhostUserFormated
				}
				item.Members = append(item.Members, &ms.ProjectMemberFormated{User: user, Role: member.Role})
			}
		}
		result = append(result, item)
	}
	return result, nil
}

func (s *projectSrv) CreateProject(actor *ms.User, input *core.ProjectCreateInput) (*ms.Project, *ms.Post, error) {
	if actor == nil || input == nil || input.Project == nil || input.Post == nil {
		return nil, nil, authz.ErrInvalid
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		user, err := lockIdentityPolicy(tx, actor, authz.PostCreate)
		if err != nil {
			return err
		}
		if err := input.Project.Create(tx); err != nil {
			return err
		}
		for _, image := range input.Images {
			image.ProjectID = input.Project.ID
			if err := tx.Create(image).Error; err != nil {
				return err
			}
		}
		for _, tag := range input.Tags {
			tag.ProjectID = input.Project.ID
			if err := tx.Create(tag).Error; err != nil {
				return err
			}
		}
		if err := tx.Create(&dbr.ProjectMember{ProjectID: input.Project.ID, UserID: actor.ID, Role: "owner"}).Error; err != nil {
			return err
		}
		input.Post.AuditStatus = ms.PostAuditApproved
		if reviewRequired(user) {
			input.Post.AuditStatus = ms.PostAuditPending
		}
		input.Post.LatestRepliedOn = time.Now().Unix()
		if _, err := input.Post.Create(tx); err != nil {
			return err
		}
		if _, err := (&dbr.PostMetric{PostId: input.Post.ID}).Create(tx); err != nil {
			return err
		}
		for _, content := range input.Contents {
			if content.Type == ms.ContentTypeLink && content.Content == "project:pending" {
				content.Content = fmt.Sprintf("project:%d", input.Project.ID)
			}
			content.PostID, content.UserID = input.Post.ID, actor.ID
			if _, err := content.Create(tx); err != nil {
				return err
			}
		}
		if input.Post.AuditStatus == ms.PostAuditApproved {
			if _, err := createTags(tx, actor.ID, reviewTags(input.Post.Tags)); err != nil {
				return err
			}
		}
		if err := enqueueReviewTx(tx, ms.ReviewPost, input.Post.ID, conf.ReviewDeadline()); err != nil {
			return err
		}
		input.Project.PostID = input.Post.ID
		return tx.Model(input.Project).Update("post_id", input.Post.ID).Error
	})
	return input.Project, input.Post, err
}

func (s *projectSrv) SetProjectFollow(projectID, userID int64, follow bool) (bool, int64, error) {
	var status bool
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var project dbr.Project
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND is_del = ?", projectID, 0).First(&project).Error; err != nil {
			return err
		}
		if follow {
			result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&dbr.ProjectFollow{ProjectID: projectID, UserID: userID, CreatedOn: time.Now().Unix()})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected > 0 {
				if err := tx.Model(&project).UpdateColumn("follow_count", gorm.Expr("follow_count + 1")).Error; err != nil {
					return err
				}
			}
			status = true
		} else {
			result := tx.Where("project_id = ? AND user_id = ?", projectID, userID).Delete(&dbr.ProjectFollow{})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected > 0 {
				if err := tx.Model(&project).UpdateColumn("follow_count", gorm.Expr("GREATEST(follow_count - 1, 0)")).Error; err != nil {
					return err
				}
			}
			status = false
		}
		return nil
	})
	if err != nil {
		return false, 0, err
	}
	var count int64
	err = s.db.Model(&dbr.Project{}).Where("id = ?", projectID).Pluck("follow_count", &count).Error
	return status, count, err
}
