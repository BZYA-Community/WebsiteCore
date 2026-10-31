package search

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/BZYA-Community/WebsiteCore/internal/application/columnindex"
)

// ColumnWriter is the HTTP adapter for an independent Meilisearch index.
type ColumnWriter struct {
	endpoint, apiKey, index string
	client                  *http.Client
}

func NewColumnWriter(endpoint, apiKey, index string, client *http.Client) *ColumnWriter {
	return &ColumnWriter{endpoint: endpoint, apiKey: apiKey, index: index, client: client}
}

func (s *ColumnWriter) request(method, path string, body, out any) error {
	var data io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		data = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, s.endpoint+path, data)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
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

func (s *ColumnWriter) task(method, path string, body any) error {
	var task struct {
		TaskUID int64 `json:"taskUid"`
	}
	if err := s.request(method, path, body, &task); err != nil {
		return err
	}
	for i := 0; i < 30; i++ {
		var result struct {
			Status string `json:"status"`
		}
		if err := s.request("GET", "/tasks/"+strconv.FormatInt(task.TaskUID, 10), nil, &result); err != nil {
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

func (s *ColumnWriter) path() string {
	return "/indexes/" + url.PathEscape(s.index)
}

func (s *ColumnWriter) Reset(docs []columnindex.Document) error {
	path := s.path()
	if err := s.request("GET", path, nil, nil); err != nil {
		if err := s.task("POST", "/indexes", map[string]any{"uid": s.index, "primaryKey": "id"}); err != nil {
			return err
		}
	}
	if err := s.task("PATCH", path+"/settings", map[string]any{"searchableAttributes": []string{"title", "summary", "body"}, "filterableAttributes": []string{"official"}, "sortableAttributes": []string{"published_on"}}); err != nil {
		return err
	}
	if err := s.task("DELETE", path+"/documents", nil); err != nil {
		return err
	}
	if len(docs) > 0 {
		return s.task("POST", path+"/documents", docs)
	}
	return nil
}

func (s *ColumnWriter) Put(doc columnindex.Document) error {
	return s.task("POST", s.path()+"/documents", []columnindex.Document{doc})
}

func (s *ColumnWriter) Delete(id int64) error {
	return s.task("DELETE", s.path()+"/documents/"+strconv.FormatInt(id, 10), nil)
}

func (s *ColumnWriter) Search(keyword string, limit int) (columnindex.Results, error) {
	var result columnindex.Results
	err := s.request("POST", s.path()+"/search", map[string]any{"q": keyword, "limit": limit, "attributesToRetrieve": []string{"id", "revision"}}, &result)
	return result, err
}
