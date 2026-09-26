// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package search

import (
	"strings"

	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/jinzhu/dbr"
	"github.com/Masterminds/semver/v3"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// sqlTweetSearchServant SQL 直查搜索实现（PostgreSQL）:
// 不依赖任何外部搜索引擎，内容检索走 post_content.content 的 ILIKE 模糊匹配，
// 标签检索走 post.tags(逗号分隔) 的 string_to_array 精确匹配。
// 作为未开启 Meili 特性时的默认回退实现，适合中小数据量部署。
type sqlTweetSearchServant struct {
	tweetSearchFilter

	db *gorm.DB
}

func (s *sqlTweetSearchServant) Name() string {
	return "SQL"
}

func (s *sqlTweetSearchServant) Version() *semver.Version {
	return semver.MustParse("v0.1.0")
}

func (s *sqlTweetSearchServant) IndexName() string {
	return "post"
}

// AddDocuments SQL 直查以数据库为唯一数据源，无索引需要维护，空操作。
func (s *sqlTweetSearchServant) AddDocuments(_ []core.TsDocItem, _primaryKey ...string) (bool, error) {
	return true, nil
}

// DeleteDocuments SQL 直查以数据库为唯一数据源，无索引需要维护，空操作。
func (s *sqlTweetSearchServant) DeleteDocuments(_ []string) error {
	return nil
}

func (s *sqlTweetSearchServant) Search(user *ms.User, q *core.QueryReq, offset, limit int) (*core.QueryResp, error) {
	var (
		resp *core.QueryResp
		err  error
	)
	switch {
	case q.Type == core.SearchTypeTag && q.Query != "":
		resp, err = s.queryByTag(user, q, offset, limit)
	case q.Query != "":
		resp, err = s.queryByContent(user, q, offset, limit)
	default:
		resp, err = s.queryAny(user, offset, limit)
	}
	if err != nil {
		logrus.Errorf("sqlTweetSearchServant.Search searchType:%s query:%s error:%v", q.Type, q.Query, err)
		return nil, err
	}
	logrus.Debugf("sqlTweetSearchServant.Search type:%s query:%s resp Hits:%d Total:%d offset:%d limit:%d", q.Type, q.Query, len(resp.Items), resp.Total, offset, limit)
	s.filterResp(user, resp)
	return resp, nil
}

// baseQuery 基础查询: 仅已过审帖子 + 按用户身份做可见性过滤（与 meili filterList 同口径）
func (s *sqlTweetSearchServant) baseQuery(user *ms.User) *gorm.DB {
	db := s.db.Model(&dbr.Post{}).Where("audit_status = ?", dbr.PostAuditApproved)
	switch {
	case user != nil && user.IsAdmin:
		// 管理员不过滤
	case user == nil:
		db = db.Where("visibility = ?", core.PostVisitPublic)
	default:
		// 好友功能已移除: 好友可见与私密同口径(仅作者本人可见)
		db = db.Where("visibility = ? OR ((visibility = ? OR visibility = ?) AND user_id = ?)",
			core.PostVisitPublic, core.PostVisitPrivate, core.PostVisitFriend, user.ID)
	}
	return db
}

func (s *sqlTweetSearchServant) queryByContent(user *ms.User, q *core.QueryReq, offset, limit int) (*core.QueryResp, error) {
	postTable := s.db.NamingStrategy.TableName("post")
	contentTable := s.db.NamingStrategy.TableName("post_content")
	kw := "%" + escapeLikeKeyword(q.Query) + "%"
	db := s.baseQuery(user).Where(
		"EXISTS (SELECT 1 FROM "+contentTable+" pc WHERE pc.post_id = "+postTable+".id AND pc.is_del = 0 AND pc.content ILIKE ?)",
		kw,
	)
	return s.queryPosts(db, offset, limit)
}

func (s *sqlTweetSearchServant) queryByTag(user *ms.User, q *core.QueryReq, offset, limit int) (*core.QueryResp, error) {
	db := s.baseQuery(user).Where("? = ANY(string_to_array(tags, ','))", q.Query)
	return s.queryPosts(db, offset, limit)
}

func (s *sqlTweetSearchServant) queryAny(user *ms.User, offset, limit int) (*core.QueryResp, error) {
	return s.queryPosts(s.baseQuery(user), offset, limit)
}

func (s *sqlTweetSearchServant) queryPosts(db *gorm.DB, offset, limit int) (*core.QueryResp, error) {
	var total int64
	if err := db.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, err
	}

	posts := make([]*dbr.Post, 0, limit)
	if err := db.Session(&gorm.Session{}).
		Order("is_top DESC, latest_replied_on DESC").
		Offset(offset).Limit(limit).
		Find(&posts).Error; err != nil {
		return nil, err
	}

	items := make([]*ms.PostFormated, 0, len(posts))
	for _, p := range posts {
		item := p.Format()
		// 与 meili 实现同口径: 查询已限定过审帖子
		item.AuditStatus = ms.PostAuditApproved
		items = append(items, item)
	}
	return &core.QueryResp{
		Items: items,
		Total: total,
	}, nil
}

// escapeLikeKeyword 转义 LIKE/ILIKE 关键词中的特殊字符(\ % _)
func escapeLikeKeyword(kw string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(kw)
}
