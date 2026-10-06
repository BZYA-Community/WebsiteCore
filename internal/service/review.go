package service

import "github.com/BZYA-Community/WebsiteCore/internal/servants/web"

func (s *webService) OnStop() error {
	web.StopReviewJobs()
	return nil
}
