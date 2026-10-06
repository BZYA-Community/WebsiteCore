package web

import "github.com/BZYA-Community/WebsiteCore/internal/core/ms"

type ReviewStatisticsReq struct {
	BaseInfo `form:"-" binding:"-"`
}
type ReviewStatisticsResp struct {
	Items []*ms.ReviewerStatistics `json:"items"`
}
type ReviewHistoryReq struct {
	BaseInfo `form:"-" binding:"-"`
	TaskID   int64 `form:"task_id" binding:"required,min=1"`
}
type ReviewHistoryResp struct {
	Items []*ms.ReviewTaskEvent `json:"items"`
}
