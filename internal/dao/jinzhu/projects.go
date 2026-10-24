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

func projectVisibleScope(db *gorm.DB, viewerID int64, viewerIsAuditor bool) *gorm.DB {
	db = db.Table(conf.DatabaseSetting.TablePrefix+"project AS project").
		Joins("JOIN "+conf.DatabaseSetting.TablePrefix+"post AS post ON post.id = project.post_id AND post.is_del = 0").
		Where("project.is_del = ?", 0)
	if !viewerIsAuditor {
		db = db.Where("post.audit_status = ? OR project.creator_id = ?", ms.PostAuditApproved, viewerID)
	}
	return db
}

func (s *projectSrv) ListProjects(viewerID int64, viewerIsAuditor bool, keyword, tag string, offset, limit int) ([]*ms.ProjectFormated, int64, error) {
	db := projectVisibleScope(s.db, viewerID, viewerIsAuditor)
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		db = db.Where("LOWER(project.title) LIKE LOWER(?)", "%"+keyword+"%")
	}
	if tag = strings.TrimSpace(tag); tag != "" {
		db = db.Where("EXISTS (SELECT 1 FROM "+conf.DatabaseSetting.TablePrefix+"project_tag pt WHERE pt.project_id = project.id AND pt.is_del = 0 AND pt.name = ?)", tag)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var projects []*dbr.Project
	if err := db.Select("project.*").Order("project.id DESC").Offset(offset).Limit(limit).Find(&projects).Error; err != nil {
		return nil, 0, err
	}
	items, err := s.formatProjects(projects, viewerID)
	return items, total, err
}

func (s *projectSrv) GetProject(viewerID int64, viewerIsAuditor bool, id int64) (*ms.ProjectFormated, error) {
	var project dbr.Project
	if err := projectVisibleScope(s.db, viewerID, viewerIsAuditor).Select("project.*").Where("project.id = ?", id).First(&project).Error; err != nil {
		return nil, err
	}
	items, err := s.formatProjects([]*dbr.Project{&project}, viewerID)
	if err != nil || len(items) == 0 {
		return nil, err
	}
	return items[0], nil
}

func (s *projectSrv) GetProjectByPostID(postID int64) (*ms.Project, error) {
	var project dbr.Project
	err := s.db.Where("post_id = ? AND is_del = ?", postID, 0).First(&project).Error
	return &project, err
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
