// Package columnindex coordinates the independent article index. Persistence,
// search transport and serialization are injected; SQL remains the read fallback.
package columnindex

import "sync"

type Document struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	Summary     string `json:"summary"`
	Body        string `json:"body"`
	Official    bool   `json:"official"`
	PublishedOn int64  `json:"published_on"`
	Revision    int64  `json:"revision"`
}

type Hit struct {
	ID       int64 `json:"id"`
	Revision int64 `json:"revision"`
}

type Results struct {
	Hits  []Hit `json:"hits"`
	Total int64 `json:"estimatedTotalHits"`
}

// Store returns only current public articles. Public returns nil for an absent
// or ineligible article; failures must be returned rather than treated as deletion.
type Store interface {
	ListPublic() ([]Document, error)
	Public(id int64) (*Document, error)
	Versions(ids []int64) (map[int64]int64, error)
}

// Writer waits for writes to finish before returning success. Reset replaces the
// entire independent index, including its settings and any stale documents.
type Writer interface {
	Reset([]Document) error
	Put(Document) error
	Delete(id int64) error
	Search(keyword string, limit int) (Results, error)
}

const SearchLimit = 1000

type Index struct {
	store  Store
	writer Writer
	lock   sync.Locker
	ready  bool
}

func New(store Store, writer Writer, lock sync.Locker) *Index {
	return &Index{store: store, writer: writer, lock: lock}
}

func (s *Index) ensure() error {
	if s.ready {
		return nil
	}
	docs, err := s.store.ListPublic()
	if err != nil {
		return err
	}
	if err = s.writer.Reset(docs); err != nil {
		return err
	}
	s.ready = true
	return nil
}

// Sync reconciles an article after its transaction commits. A failure invalidates
// readiness so the next operation rebuilds from public database content.
func (s *Index) Sync(id int64) (err error) {
	if s == nil {
		return nil
	}
	s.lock.Lock()
	defer s.lock.Unlock()
	defer func() {
		if err != nil {
			s.ready = false
		}
	}()
	if err = s.ensure(); err != nil {
		return err
	}
	doc, err := s.store.Public(id)
	if err != nil {
		return err
	}
	if doc == nil {
		return s.writer.Delete(id)
	}
	return s.writer.Put(*doc)
}

// Search returns usable=false for disabled, unavailable, empty, capped or stale
// results. Callers then use their SQL keyword query. Stale results and failures
// invalidate readiness, triggering reconciliation on the next operation.
func (s *Index) Search(keyword string) (ids []int64, usable bool) {
	if s == nil {
		return nil, false
	}
	s.lock.Lock()
	defer s.lock.Unlock()
	if err := s.ensure(); err != nil {
		s.ready = false
		return nil, false
	}
	results, err := s.writer.Search(keyword, SearchLimit)
	if err != nil {
		s.ready = false
		return nil, false
	}
	if results.Total >= SearchLimit || len(results.Hits) == 0 {
		return nil, false
	}
	ids = make([]int64, 0, len(results.Hits))
	for _, hit := range results.Hits {
		ids = append(ids, hit.ID)
	}
	versions, err := s.store.Versions(ids)
	if err != nil {
		return nil, false
	}
	for _, hit := range results.Hits {
		if revision, exists := versions[hit.ID]; !exists || revision != hit.Revision {
			s.ready = false
			return nil, false
		}
	}
	return ids, true
}
