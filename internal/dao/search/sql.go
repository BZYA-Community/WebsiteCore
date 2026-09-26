package search

import (
	"strings"

	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/jinzhu/dbr"
	"github.com/Masterminds/semver/v3"
	"gorm.io/gorm"
)

// SQL search reads current posts directly; index updates are unnecessary.
type sqlTweetSearchServant struct{ db *gorm.DB }

func NewSQLTweetSearchService(db *gorm.DB) (core.TweetSearchService, core.VersionInfo) {
	s := &sqlTweetSearchServant{db: db}
	return s, s
}
func (s *sqlTweetSearchServant) Name() string             { return "SQL" }
func (s *sqlTweetSearchServant) Version() *semver.Version { return semver.MustParse("1.0.0") }
func (s *sqlTweetSearchServant) IndexName() string        { return "" }
func (s *sqlTweetSearchServant) AddDocuments([]core.TsDocItem, ...string) (bool, error) {
	return true, nil
}
func (s *sqlTweetSearchServant) DeleteDocuments([]string) error { return nil }

func (s *sqlTweetSearchServant) Search(user *ms.User, q *core.QueryReq, offset, limit int) (*core.QueryResp, error) {
	db := s.db.Model(&dbr.Post{}).Where("is_del = ? AND audit_status = ?", 0, ms.PostAuditApproved)
	if user == nil {
		db = db.Where("visibility = ?", core.PostVisitPublic)
	} else if !user.IsAdmin {
		db = db.Where("visibility = ? OR (visibility IN ? AND user_id = ?)", core.PostVisitPublic, []core.PostVisibleT{core.PostVisitPrivate, core.PostVisitFriend}, user.ID)
	}
	if q.Query != "" {
		// Escape LIKE metacharacters so queries match literal substrings.
		escaped := strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(strings.ToLower(q.Query))
		switch q.Type {
		case core.SearchTypeTag:
			tags := "LOWER(',' || tags || ',')"
			if s.db.Dialector.Name() == "mysql" {
				tags = "LOWER(CONCAT(',', tags, ','))"
			}
			db = db.Where(tags+" LIKE ? ESCAPE '!'", "%,"+escaped+",%")
		case core.SearchTypeDefault:
			contents := s.db.Model(&dbr.PostContent{}).Select("post_id").Where("is_del = ? AND type IN ?", 0, []dbr.PostContentT{dbr.ContentTypeTitle, dbr.ContentTypeText, dbr.ContentTypeMarkdown}).Where("LOWER(content) LIKE ? ESCAPE '!'", "%"+escaped+"%")
			db = db.Where("id IN (?)", contents)
		}
	}
	resp := &core.QueryResp{Items: make([]*ms.PostFormated, 0)}
	if err := db.Count(&resp.Total).Error; err != nil {
		return nil, err
	}
	var posts []*dbr.Post
	if err := db.Order("is_top DESC, latest_replied_on DESC, id DESC").Offset(offset).Limit(limit).Find(&posts).Error; err != nil {
		return nil, err
	}
	for _, post := range posts {
		resp.Items = append(resp.Items, post.Format())
	}
	return resp, nil
}
