package jinzhu

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/BZYA-Community/WebsiteCore/internal/conf"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/alimy/tryst/cfg"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// One independent index per configured post index. PostgreSQL remains the source
// of truth for visibility, including when a search service has stale documents.
var columnSearch = struct {
	sync.Mutex
	ready  bool
	client *http.Client
}{client: &http.Client{Timeout: 3 * time.Second}}

func columnMeili(method, path string, body, out any) error {
	var data io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		data = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, conf.MeiliSetting.Endpoint()+path, data)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+conf.MeiliSetting.ApiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := columnSearch.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("column search HTTP %d", resp.StatusCode)
	}
	if out != nil {
		return json.NewDecoder(io.LimitReader(resp.Body, 16<<20)).Decode(out)
	}
	return nil
}

func columnMeiliTask(method, path string, body any) error {
	var task struct {
		TaskUID int64 `json:"taskUid"`
	}
	if err := columnMeili(method, path, body, &task); err != nil {
		return err
	}
	for i := 0; i < 30; i++ {
		var result struct {
			Status string `json:"status"`
		}
		if err := columnMeili("GET", "/tasks/"+strconv.FormatInt(task.TaskUID, 10), nil, &result); err != nil {
			return err
		}
		if result.Status == "succeeded" {
			return nil
		}
		if result.Status == "failed" || result.Status == "canceled" {
			return fmt.Errorf("column search task %s", result.Status)
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("column search task did not finish")
}

func columnIndexPath() string {
	return "/indexes/" + url.PathEscape(conf.MeiliSetting.Index+"_columns")
}

func ensureColumnIndex(db *gorm.DB) error {
	if columnSearch.ready {
		return nil
	}
	path := columnIndexPath()
	if err := columnMeili("GET", path, nil, nil); err != nil {
		if err := columnMeiliTask("POST", "/indexes", map[string]any{"uid": conf.MeiliSetting.Index + "_columns", "primaryKey": "id"}); err != nil {
			return err
		}
	}
	if err := columnMeiliTask("PATCH", path+"/settings", map[string]any{"searchableAttributes": []string{"title", "summary", "body"}, "filterableAttributes": []string{"official"}, "sortableAttributes": []string{"published_on"}}); err != nil {
		return err
	}
	var rows []*ms.Column
	if err := publicColumns(db.Where("kind=?", ms.ColumnArticle)).Find(&rows).Error; err != nil {
		return err
	}
	if err := columnMeiliTask("DELETE", path+"/documents", nil); err != nil {
		return err
	}
	if len(rows) > 0 {
		if err := columnMeiliTask("POST", path+"/documents", columnSearchDocuments(rows)); err != nil {
			return err
		}
	}
	columnSearch.ready = true
	return nil
}

func columnSearchDocuments(rows []*ms.Column) []map[string]any {
	docs := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		docs = append(docs, map[string]any{"id": r.ID, "title": r.Title, "summary": r.Summary, "body": r.Body, "official": r.Official, "published_on": r.PublishedOn, "revision": r.PublicSubmissionID})
	}
	return docs
}

func syncColumnIndex(db *gorm.DB, id int64) {
	if !cfg.If("Meili") || conf.MeiliSetting == nil {
		return
	}
	columnSearch.Lock()
	defer columnSearch.Unlock()
	err := ensureColumnIndex(db)
	if err == nil {
		row, readErr := loadColumn(db, id)
		if readErr == nil && row.Kind == ms.ColumnArticle && columnPublic(row) {
			err = columnMeiliTask("POST", columnIndexPath()+"/documents", columnSearchDocuments([]*ms.Column{row}))
		} else {
			err = columnMeiliTask("DELETE", columnIndexPath()+"/documents/"+strconv.FormatInt(id, 10), nil)
		}
	}
	if err != nil {
		columnSearch.ready = false
		logrus.WithError(err).Warn("column search refresh failed; using database fallback")
	}
}

func searchColumnIDs(db *gorm.DB, keyword string) ([]int64, bool) {
	if !cfg.If("Meili") || conf.MeiliSetting == nil {
		return nil, false
	}
	columnSearch.Lock()
	defer columnSearch.Unlock()
	if err := ensureColumnIndex(db); err != nil {
		columnSearch.ready = false
		return nil, false
	}
	var response struct {
		Hits []struct {
			ID       int64 `json:"id"`
			Revision int64 `json:"revision"`
		} `json:"hits"`
		EstimatedTotalHits int64 `json:"estimatedTotalHits"`
	}
	if err := columnMeili("POST", columnIndexPath()+"/search", map[string]any{"q": keyword, "limit": 1000, "attributesToRetrieve": []string{"id", "revision"}}, &response); err != nil {
		columnSearch.ready = false
		return nil, false
	}
	// Avoid silently truncating results at Meilisearch's pagination ceiling.
	if response.EstimatedTotalHits >= 1000 {
		return nil, false
	}
	ids := make([]int64, 0, len(response.Hits))
	for _, hit := range response.Hits {
		ids = append(ids, hit.ID)
	}
	if len(ids) == 0 {
		return nil, false
	}
	var current []*ms.Column
	if err := publicColumns(db.Select("id", "public_submission_id").Where("id IN ?", ids)).Find(&current).Error; err != nil {
		return nil, false
	}
	versions := make(map[int64]int64, len(current))
	for _, row := range current {
		versions[row.ID] = row.PublicSubmissionID
	}
	for _, hit := range response.Hits {
		if version, exists := versions[hit.ID]; !exists || version != hit.Revision {
			columnSearch.ready = false
			return nil, false
		}
	}
	return ids, true
}
