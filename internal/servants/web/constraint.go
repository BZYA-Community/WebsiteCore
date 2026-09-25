//go:build constraint

package web

import (
	api "github.com/BZYA-Community/WebsiteCore/auto/api/v1"
)

var (
	_ api.Admin       = (*adminSrv)(nil)
	_ api.Core        = (*coreSrv)(nil)
	_ api.CourseLoose = (*courseLooseSrv)(nil)
	_ api.CoursePriv  = (*coursePrivSrv)(nil)
	_ api.CourseAdmin = (*courseAdminSrv)(nil)
	_ api.Followship  = (*followshipSrv)(nil)
	_ api.Loose       = (*looseSrv)(nil)
	_ api.Pub         = (*pubSrv)(nil)
	_ api.Relax       = (*relaxSrv)(nil)
	_ api.Site        = (*siteSrv)(nil)
	_ api.Trends      = (*trendsSrv)(nil)
	_ api.Priv        = (*privSrv)(nil)
	_ api.PrivChain   = (*privChain)(nil)
)
