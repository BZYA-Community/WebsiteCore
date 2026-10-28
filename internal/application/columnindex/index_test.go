package columnindex_test

import (
	"errors"
	"reflect"
	"slices"
	"sync"
	"testing"

	"github.com/BZYA-Community/WebsiteCore/internal/application/columnindex"
)

type memoryStore struct {
	docs []columnindex.Document
	err  error
}

func (s *memoryStore) ListPublic() ([]columnindex.Document, error) { return s.docs, s.err }
func (s *memoryStore) Public(id int64) (*columnindex.Document, error) {
	if s.err != nil {
		return nil, s.err
	}
	for _, doc := range s.docs {
		if doc.ID == id {
			return &doc, nil
		}
	}
	return nil, nil
}
func (s *memoryStore) Versions(ids []int64) (map[int64]int64, error) {
	versions := make(map[int64]int64)
	for _, id := range ids {
		for _, doc := range s.docs {
			if doc.ID == id {
				versions[id] = doc.Revision
			}
		}
	}
	return versions, s.err
}

type memoryWriter struct {
	resets  int
	docs    []columnindex.Document
	puts    []columnindex.Document
	deletes []int64
	results columnindex.Results
	err     error
	keyword string
	limit   int
}

func (w *memoryWriter) Reset(docs []columnindex.Document) error {
	w.resets++
	w.docs = append([]columnindex.Document(nil), docs...)
	return w.err
}
func (w *memoryWriter) Put(doc columnindex.Document) error {
	w.puts = append(w.puts, doc)
	return w.err
}
func (w *memoryWriter) Delete(id int64) error { w.deletes = append(w.deletes, id); return w.err }
func (w *memoryWriter) Search(keyword string, limit int) (columnindex.Results, error) {
	w.keyword, w.limit = keyword, limit
	return w.results, w.err
}
func fixture() (*memoryStore, *memoryWriter, *columnindex.Index) {
	store := &memoryStore{docs: []columnindex.Document{{ID: 1, Title: "Current title", Summary: "summary", Body: "public body", Official: true, PublishedOn: 123, Revision: 7}}}
	writer := &memoryWriter{results: columnindex.Results{Hits: []columnindex.Hit{{ID: 1, Revision: 7}}, Total: 1}}
	return store, writer, columnindex.New(store, writer, &sync.Mutex{})
}
func TestSearchRebuildsOnceAndValidatesPublicRevisions(t *testing.T) {
	store, writer, index := fixture()
	for range 2 {
		ids, ok := index.Search("public")
		if !ok || !reflect.DeepEqual(ids, []int64{1}) {
			t.Fatalf("search = %v, %v", ids, ok)
		}
	}
	if writer.resets != 1 || !slices.Equal(writer.docs, store.docs) || writer.keyword != "public" || writer.limit != columnindex.SearchLimit {
		t.Fatalf("unexpected projection or search request: %+v", writer)
	}
	// A stale revision or deleted/private hit must force SQL fallback, then repair.
	for _, docs := range [][]columnindex.Document{{{ID: 1, Revision: 8}}, {}} {
		store.docs = docs
		if _, ok := index.Search("public"); ok {
			t.Fatal("stale hit accepted")
		}
		before := writer.resets
		writer.results = columnindex.Results{}
		if _, ok := index.Search("public"); ok {
			t.Fatal("empty results must fall back")
		}
		if writer.resets != before+1 || !slices.Equal(writer.docs, store.docs) {
			t.Fatal("stale index was not rebuilt")
		}
		writer.results = columnindex.Results{Hits: []columnindex.Hit{{ID: 1, Revision: 8}}, Total: 1}
	}
}
func TestSearchFallbackAndRecovery(t *testing.T) {
	failure := errors.New("unavailable")
	for _, name := range []string{"disabled", "store", "reset", "search", "versions", "empty", "capped"} {
		t.Run(name, func(t *testing.T) {
			store, writer, index := fixture()
			if name == "disabled" {
				index = nil
			}
			if name == "store" {
				store.err = failure
			}
			if name == "reset" {
				writer.err = failure
			}
			if name == "search" || name == "versions" {
				if _, ok := index.Search("body"); !ok {
					t.Fatal("initial search failed")
				}
				if name == "search" {
					writer.err = failure
				} else {
					store.err = failure
				}
			}
			if name == "empty" {
				writer.results = columnindex.Results{}
			}
			if name == "capped" {
				writer.results.Total = columnindex.SearchLimit
			}
			if _, ok := index.Search("body"); ok {
				t.Fatal("fallback not requested")
			}
			if name == "disabled" {
				return
			}
			store.err, writer.err = nil, nil
			writer.results = columnindex.Results{Hits: []columnindex.Hit{{ID: 1, Revision: 7}}, Total: 1}
			if _, ok := index.Search("body"); !ok {
				t.Fatal("failed to recover")
			}
		})
	}
}
func TestSyncUpdatesDeletesAndRetriesFailedWrites(t *testing.T) {
	store, writer, index := fixture()
	if err := index.Sync(1); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(writer.puts, store.docs) {
		t.Fatal("sync lost document fields")
	}
	store.docs = nil
	if err := index.Sync(1); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(writer.deletes, []int64{1}) {
		t.Fatal("nonpublic article not deleted")
	}
	store.err = errors.New("database offline")
	if err := index.Sync(1); err == nil {
		t.Fatal("database failure hidden")
	}
	if len(writer.deletes) != 1 {
		t.Fatal("database failure incorrectly deleted document")
	}
	store.err = nil
	writer.err = errors.New("search offline")
	if err := index.Sync(1); err == nil {
		t.Fatal("reset failure hidden")
	}
	writer.err = nil
	before := writer.resets
	if err := index.Sync(1); err != nil {
		t.Fatal(err)
	}
	if writer.resets != before+1 {
		t.Fatal("failed sync did not invalidate readiness")
	}
	writer.err = errors.New("write failed")
	if err := index.Sync(1); err == nil {
		t.Fatal("delete failure hidden")
	}
	writer.err = nil
	before = writer.resets
	if err := index.Sync(1); err != nil {
		t.Fatal(err)
	}
	if writer.resets != before+1 {
		t.Fatal("failed deletion did not trigger rebuild")
	}
	var disabled *columnindex.Index
	if err := disabled.Sync(1); err != nil {
		t.Fatal(err)
	}
}
