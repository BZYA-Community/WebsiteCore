package search

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/BZYA-Community/WebsiteCore/internal/application/columnindex"
)

type columnRoundTrip func(*http.Request) (*http.Response, error)

func (f columnRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestColumnWriterProtocol(t *testing.T) {
	var requests []string
	var indexed []columnindex.Document
	var searchRequest map[string]any
	client := &http.Client{Transport: columnRoundTrip(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "search.example" || r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatal("writer ignored injected configuration")
		}
		requests = append(requests, r.Method+" "+r.URL.Path)
		body, status := `{"taskUid":4}`, http.StatusAccepted
		switch {
		case r.URL.Path == "/tasks/4":
			body, status = `{"status":"succeeded"}`, http.StatusOK
		case r.Method == "GET":
			body, status = `{}`, http.StatusNotFound
		case r.URL.Path == "/indexes":
			var config map[string]string
			if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
				t.Fatal(err)
			}
			if config["uid"] != "forum_columns" || config["primaryKey"] != "id" {
				t.Fatalf("index config = %v", config)
			}
		case r.Method == "PATCH":
			var settings map[string][]string
			if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(settings["searchableAttributes"], []string{"title", "summary", "body"}) {
				t.Fatalf("settings = %v", settings)
			}
		case r.URL.Path == "/indexes/forum_columns/search":
			if err := json.NewDecoder(r.Body).Decode(&searchRequest); err != nil {
				t.Fatal(err)
			}
			body, status = `{"hits":[{"id":2,"revision":9}],"estimatedTotalHits":1}`, http.StatusOK
		case r.Method == "POST":
			if err := json.NewDecoder(r.Body).Decode(&indexed); err != nil {
				t.Fatal(err)
			}
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	writer := NewColumnWriter("http://search.example", "test-key", "forum_columns", client)
	doc := columnindex.Document{ID: 2, Title: "Title", Body: "body", Revision: 9}
	if err := writer.Reset([]columnindex.Document{doc}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(indexed, []columnindex.Document{doc}) {
		t.Fatal("reset lost document projection")
	}
	if err := writer.Put(doc); err != nil {
		t.Fatal(err)
	}
	if err := writer.Delete(2); err != nil {
		t.Fatal(err)
	}
	results, err := writer.Search("keyword", 1000)
	if err != nil || results.Total != 1 || len(results.Hits) != 1 || results.Hits[0].Revision != 9 {
		t.Fatalf("results=%+v err=%v", results, err)
	}
	if searchRequest["q"] != "keyword" || searchRequest["limit"] != float64(1000) {
		t.Fatalf("search request = %v", searchRequest)
	}
	want := []string{"GET /indexes/forum_columns", "POST /indexes", "GET /tasks/4", "PATCH /indexes/forum_columns/settings", "GET /tasks/4", "DELETE /indexes/forum_columns/documents", "GET /tasks/4", "POST /indexes/forum_columns/documents", "GET /tasks/4", "POST /indexes/forum_columns/documents", "GET /tasks/4", "DELETE /indexes/forum_columns/documents/2", "GET /tasks/4", "POST /indexes/forum_columns/search"}
	if !reflect.DeepEqual(requests, want) {
		t.Fatalf("requests=%v", requests)
	}
}

func TestColumnWriterReturnsTransportAndTaskFailures(t *testing.T) {
	for _, failure := range []string{"transport", "http", "failed", "canceled"} {
		t.Run(failure, func(t *testing.T) {
			client := &http.Client{Transport: columnRoundTrip(func(r *http.Request) (*http.Response, error) {
				if failure == "transport" {
					return nil, fmt.Errorf("offline")
				}
				status, body := http.StatusAccepted, `{"taskUid":5}`
				if failure == "http" {
					status = http.StatusServiceUnavailable
				}
				if r.URL.Path == "/tasks/5" {
					status, body = http.StatusOK, `{"status":"`+failure+`"}`
				}
				return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
			})}
			writer := NewColumnWriter("http://search.example", "test-key", "forum_columns", client)
			if err := writer.Delete(2); err == nil {
				t.Fatal("writer hid failure")
			}
		})
	}
}
