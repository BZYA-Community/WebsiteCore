package web

import (
	"errors"

	"github.com/BZYA-Community/WebsiteCore/internal/authz"
	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/internal/model/web"
	"github.com/BZYA-Community/WebsiteCore/pkg/xerror"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func reviewError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, authz.ErrDenied):
		return web.ErrNoPermission
	case errors.Is(err, authz.ErrInvalid):
		return xerror.InvalidParams
	case errors.Is(err, core.ErrReviewStale):
		return xerror.InvalidParams.WithDetails("审核任务已变化，请刷新后重试")
	case errors.Is(err, gorm.ErrRecordNotFound):
		return web.ErrNoPermission
	default:
		logrus.WithError(err).Error("review operation failed")
		return xerror.ServerError
	}
}

func (s *auditSrv) decideTarget(actor *ms.User, kind string, targetID, taskID, revision int64, action, reason string) (*core.ReviewDecision, error) {
	// Bind the request's target to its revisioned task; a task for another item
	// cannot authorize an action simply by being present in the request body.
	task, err := s.Ds.ReviewTaskForTarget(actor, kind, targetID)
	if err != nil {
		return nil, reviewError(err)
	}
	if task.ID != taskID {
		return nil, web.ErrNoPermission
	}
	result, err := s.Ds.DecideReview(actor, taskID, revision, action, reason)
	return result, reviewError(err)
}

func (s *auditSrv) ReviewStatistics(req *web.ReviewStatisticsReq) (*web.ReviewStatisticsResp, error) {
	stats, err := s.Ds.ReviewerTimeoutStatistics(req.User)
	if err != nil {
		return nil, reviewError(err)
	}
	return &web.ReviewStatisticsResp{Items: stats}, nil
}

func (s *auditSrv) ReviewHistory(req *web.ReviewHistoryReq) (*web.ReviewHistoryResp, error) {
	events, err := s.Ds.ListReviewTaskEvents(req.User, req.TaskID)
	if err != nil {
		return nil, reviewError(err)
	}
	return &web.ReviewHistoryResp{Items: events}, nil
}
