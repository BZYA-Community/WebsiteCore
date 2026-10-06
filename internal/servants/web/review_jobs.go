package web

import (
	"sync"
	"time"

	"github.com/BZYA-Community/WebsiteCore/internal/conf"
	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
)

var reviewJobs struct {
	sync.Mutex
	manager *cron.Cron
}

// Review deadlines remain active even when optional analytics jobs are disabled.
func scheduleReviewJobs() {
	reviewJobs.Lock()
	defer reviewJobs.Unlock()
	if reviewJobs.manager != nil {
		return
	}
	interval := time.Minute
	if conf.AuditSetting != nil && conf.AuditSetting.AssignmentIntervalSeconds > 0 {
		interval = time.Duration(conf.AuditSetting.AssignmentIntervalSeconds) * time.Second
	}
	run := func() {
		if err := _ds.ReconcileReviewTasks(conf.ReviewDeadline()); err != nil {
			logrus.Errorf("review assignment reconciliation failed: %v", err)
		}
	}
	manager := cron.New(cron.WithChain(cron.SkipIfStillRunning(cron.DefaultLogger)))
	manager.Schedule(cron.Every(interval), cron.FuncJob(run))
	manager.Schedule(cron.Every(time.Hour), cron.FuncJob(func() {
		if err := _ds.ExpireCourseUploads(_oss); err != nil {
			logrus.WithError(err).Error("course upload cleanup failed")
		}
	}))
	reviewJobs.manager = manager
	run()
	manager.Start()
}

func StopReviewJobs() {
	reviewJobs.Lock()
	manager := reviewJobs.manager
	reviewJobs.manager = nil
	reviewJobs.Unlock()
	if manager != nil {
		<-manager.Stop().Done()
	}
}
