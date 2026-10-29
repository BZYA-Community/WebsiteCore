package jinzhu

import (
	"errors"

	"github.com/BZYA-Community/WebsiteCore/internal/application/columnindex"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// columnIndexStore supplies only published article projections to the module.
type columnIndexStore struct{ db *gorm.DB }

func columnDocument(row *ms.Column) columnindex.Document {
	return columnindex.Document{ID: row.ID, Title: row.Title, Summary: row.Summary, Body: row.Body, Official: row.Official, PublishedOn: row.PublishedOn, Revision: row.PublicSubmissionID}
}

func (s columnIndexStore) ListPublic() ([]columnindex.Document, error) {
	var rows []*ms.Column
	if err := publicColumns(s.db.Where("kind=?", ms.ColumnArticle)).Find(&rows).Error; err != nil {
		return nil, err
	}
	docs := make([]columnindex.Document, 0, len(rows))
	for _, row := range rows {
		docs = append(docs, columnDocument(row))
	}
	return docs, nil
}

func (s columnIndexStore) Public(id int64) (*columnindex.Document, error) {
	row, err := loadColumn(s.db, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if row.Kind != ms.ColumnArticle || !columnPublic(row) {
		return nil, nil
	}
	doc := columnDocument(row)
	return &doc, nil
}

func (s columnIndexStore) Versions(ids []int64) (map[int64]int64, error) {
	var rows []*ms.Column
	if err := publicColumns(s.db.Select("id", "public_submission_id").Where("kind=? AND id IN ?", ms.ColumnArticle, ids)).Find(&rows).Error; err != nil {
		return nil, err
	}
	versions := make(map[int64]int64, len(rows))
	for _, row := range rows {
		versions[row.ID] = row.PublicSubmissionID
	}
	return versions, nil
}

func syncColumnIndex(index *columnindex.Index, id int64) {
	if err := index.Sync(id); err != nil {
		logrus.WithError(err).Warn("column search refresh failed; using database fallback")
	}
}
