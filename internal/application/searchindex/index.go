// Package searchindex prepares and synchronizes post documents. Event dispatch
// and concrete database, lock, and search adapters belong to the caller.
package searchindex

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
)

// Store supplies the same eligible posts and enrichment used by existing syncs.
type Store interface {
	ListSyncSearchTweets(limit, offset int) ([]*ms.Post, int64, error)
	MergePosts(posts []*ms.Post) ([]*ms.PostFormated, error)
	GetPostContentsByIDs(ids []int64) ([]*ms.PostContent, error)
}

type Writer interface {
	AddDocuments(data []core.TsDocItem, primaryKey ...string) (bool, error)
	DeleteDocuments(identifiers []string) error
}

// Lock serializes full sync jobs. Production uses the existing Redis job lock.
type Lock interface {
	SetPushToSearchJob(ctx context.Context) error
	DelPushToSearchJob(ctx context.Context) error
}

type Index struct {
	store  Store
	writer Writer
	lock   Lock
}

func New(store Store, writer Writer, lock Lock) *Index {
	return &Index{store: store, writer: writer, lock: lock}
}

// Sync indexes the snapshot page count reported by the first query. It always
// releases an acquired lock, and returns failures without retrying indefinitely.
// Successful writes mean acceptance by the writer (which may itself be queued).
func (s *Index) Sync(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.lock.SetPushToSearchJob(ctx); err != nil {
		return fmt.Errorf("redis: set JOB_PUSH_TO_SEARCH error: %w", err)
	}
	defer s.lock.DelPushToSearchJob(context.WithoutCancel(ctx))
	const pageSize = 1000
	var total int64
	for page := 0; ; page++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		posts, count, err := s.store.ListSyncSearchTweets(pageSize, page*pageSize)
		if err != nil {
			return fmt.Errorf("list search posts at offset %d: %w", page*pageSize, err)
		}
		if page == 0 {
			total = count
		}
		formatted, err := s.store.MergePosts(posts)
		if err != nil {
			return fmt.Errorf("merge search posts: %w", err)
		}
		if len(formatted) != len(posts) {
			return fmt.Errorf("merge search posts: got %d views for %d posts", len(formatted), len(posts))
		}
		for i, post := range posts {
			if err := s.write(post, formatted[i].Contents); err != nil {
				return err
			}
		}
		if int64(page+1)*pageSize >= total {
			return nil
		}
	}
}

// Put uses the same text projection as a full sync.
func (s *Index) Put(post *ms.Post) error {
	contents, err := s.store.GetPostContentsByIDs([]int64{post.ID})
	if err != nil {
		return fmt.Errorf("get search post contents: %w", err)
	}
	formatted := make([]*ms.PostContentFormated, 0, len(contents))
	for _, item := range contents {
		formatted = append(formatted, item.Format())
	}
	return s.write(post, formatted)
}

func (s *Index) Delete(post *ms.Post) error {
	return s.writer.DeleteDocuments([]string{strconv.FormatInt(post.ID, 10)})
}

func (s *Index) write(post *ms.Post, contents []*ms.PostContentFormated) error {
	var text strings.Builder
	for _, item := range contents {
		switch item.Type {
		case ms.ContentTypeText, ms.ContentTypeTitle, ms.ContentTypeMarkdown:
			text.WriteString(item.Content)
			text.WriteByte('\n')
		}
	}
	_, err := s.writer.AddDocuments([]core.TsDocItem{{Post: post, Content: text.String()}}, strconv.FormatInt(post.ID, 10))
	return err
}
