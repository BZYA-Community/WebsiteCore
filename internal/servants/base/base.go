// Package base composes shared application capabilities for HTTP servants.
package base

import (
	"github.com/BZYA-Community/WebsiteCore/internal/application/content"
	"github.com/BZYA-Community/WebsiteCore/internal/application/searchindex"
	"github.com/BZYA-Community/WebsiteCore/internal/conf"
	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/dao"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/cache"
	"github.com/BZYA-Community/WebsiteCore/internal/transport/httpx"
)

type BaseServant = httpx.Servant

type DaoServant struct {
	*BaseServant
	*content.Views
	index *searchindex.Index

	Dsa   core.WebDataServantA
	Ds    core.DataService
	Ts    core.TweetSearchService
	Redis core.RedisCache
}

func NewBaseServant() *BaseServant { return httpx.New(conf.UseSentryGin()) }

func NewDaoServant() *DaoServant {
	ds := dao.DataService()
	ts := dao.TweetSearchService()
	redis := cache.NewRedisCache()
	return &DaoServant{
		BaseServant: NewBaseServant(),
		Redis:       redis,
		Dsa:         dao.WebDataServantA(),
		Ds:          ds,
		Views:       content.New(ds),
		Ts:          ts,
		index:       searchindex.New(ds, ts, redis),
	}
}
