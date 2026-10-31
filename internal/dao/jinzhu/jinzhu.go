// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// package jinzhu Core service implement base gorm+mysql/postgresql.
// Jinzhu is the primary developer of gorm so use his name as
// package name as a saluter.

package jinzhu

import (
	"net/http"
	"sync"
	"time"

	"github.com/BZYA-Community/WebsiteCore/internal/application/columnindex"
	"github.com/BZYA-Community/WebsiteCore/internal/conf"
	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/cache"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/search"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/security"
	"github.com/Masterminds/semver/v3"
	"github.com/alimy/tryst/cfg"
)

var _onceInitial sync.Once

type dataSrv struct {
	core.ColumnService
	core.MessageService
	core.TopicService
	core.TweetService
	core.TweetManageService
	core.TweetHelpService
	core.TweetMetricServantA
	core.CommentService
	core.CommentManageService
	core.CommentMetricServantA
	core.CourseService
	core.CourseManageService
	core.CourseUploadService
	core.ProjectService
	core.TrendsManageServantA
	core.UserManageService
	core.IdentityService
	core.UserMetricServantA
	core.FollowingManageService
	core.UserRelationService
	core.SiteAdminService
	core.ReviewService
	core.SecurityService
	core.AttachmentCheckService
}

type webDataSrvA struct {
	core.TopicServantA
	core.TweetServantA
	core.TweetManageServantA
	core.TweetHelpServantA
}

func NewDataService() (core.DataService, core.VersionInfo) {
	lazyInitial()
	db := conf.MustGormDB()
	pvs := security.NewPhoneVerifyService()
	var columnsIndex *columnindex.Index
	if cfg.If("Meili") && conf.MeiliSetting != nil {
		settings := conf.MeiliSetting
		writer := search.NewColumnWriter(settings.Endpoint(), settings.ApiKey, settings.Index+"_columns", &http.Client{Timeout: 3 * time.Second})
		columnsIndex = columnindex.New(columnIndexStore{db: db}, writer, &sync.Mutex{})
	}
	tms := newTweetMetricServentA(db)
	ums := newUserMetricServentA(db)
	cms := newCommentMetricServentA(db)
	cis := cache.NewEventCacheIndexSrv(tms)
	ds := &dataSrv{
		ColumnService:          &columnSrv{db: db, index: columnsIndex},
		TweetMetricServantA:    tms,
		CommentMetricServantA:  cms,
		UserMetricServantA:     ums,
		MessageService:         newMessageService(db),
		TopicService:           newTopicService(db),
		TweetService:           newTweetService(db),
		TweetManageService:     newTweetManageService(db, cis),
		TweetHelpService:       newTweetHelpService(db),
		CommentService:         newCommentService(db),
		CommentManageService:   newCommentManageService(db),
		CourseService:          newCourseService(db),
		CourseManageService:    newCourseManageService(db),
		CourseUploadService:    &courseUploadSrv{db: db},
		ProjectService:         newProjectService(db),
		TrendsManageServantA:   newTrendsManageServentA(db),
		UserManageService:      newUserManageService(db, ums),
		IdentityService:        newIdentityService(db),
		SiteAdminService:       newAuditService(db),
		ReviewService:          newReviewService(db, columnsIndex),
		FollowingManageService: newFollowingManageService(db),
		UserRelationService:    newUserRelationService(db),
		SecurityService:        newSecurityService(db, pvs),
		AttachmentCheckService: security.NewAttachmentCheckService(),
	}
	return cache.NewCacheDataService(ds), ds
}

func NewWebDataServantA() (core.WebDataServantA, core.VersionInfo) {
	lazyInitial()
	db := conf.MustGormDB()
	ds := &webDataSrvA{
		TopicServantA:       newTopicServantA(db),
		TweetServantA:       newTweetServantA(db),
		TweetManageServantA: newTweetManageServantA(db),
		TweetHelpServantA:   newTweetHelpServantA(db),
	}
	return ds, ds
}

func (s *dataSrv) Name() string {
	return "Gorm"
}

func (s *dataSrv) Version() *semver.Version {
	return semver.MustParse("v0.2.0")
}

func (s *webDataSrvA) Name() string {
	return "Gorm"
}

func (s *webDataSrvA) Version() *semver.Version {
	return semver.MustParse("v0.1.0")
}

// lazyInitial do some package lazy initialize for performance
func lazyInitial() {
	_onceInitial.Do(func() {
		initTableName()
	})
}
