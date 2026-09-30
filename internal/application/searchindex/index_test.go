package searchindex_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/BZYA-Community/WebsiteCore/internal/application/searchindex"
	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
)

type memoryStore struct {
	posts    []*ms.Post
	contents []*ms.PostContent
	err      error
	mismatch bool
}

func (s *memoryStore) ListSyncSearchTweets(limit, offset int) ([]*ms.Post, int64, error) {
	total := len(s.posts)
	if offset >= total {
		return nil, int64(total), s.err
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return s.posts[offset:end], int64(total), s.err
}
func (s *memoryStore) MergePosts(posts []*ms.Post) ([]*ms.PostFormated, error) {
	if s.mismatch {
		return nil, nil
	}
	result := make([]*ms.PostFormated, 0, len(posts))
	for _, post := range posts {
		item := post.Format()
		for _, c := range s.contents {
			if c.PostID == post.ID {
				item.Contents = append(item.Contents, c.Format())
			}
		}
		result = append(result, item)
	}
	return result, s.err
}
func (s *memoryStore) GetPostContentsByIDs(ids []int64) ([]*ms.PostContent, error) {
	var result []*ms.PostContent
	for _, c := range s.contents {
		for _, id := range ids {
			if c.PostID == id {
				result = append(result, c)
			}
		}
	}
	return result, s.err
}

type memoryWriter struct {
	docs       []core.TsDocItem
	deleted    []string
	err        error
	afterWrite func()
}

func (s *memoryWriter) AddDocuments(docs []core.TsDocItem, _ ...string) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	s.docs = append(s.docs, docs...)
	if s.afterWrite != nil {
		s.afterWrite()
	}
	return true, nil
}
func (s *memoryWriter) DeleteDocuments(ids []string) error {
	s.deleted = append(s.deleted, ids...)
	return s.err
}

type memoryLock struct {
	held bool
	err  error
}

func (l *memoryLock) SetPushToSearchJob(context.Context) error {
	if l.err != nil {
		return l.err
	}
	if l.held {
		return errors.New("already locked")
	}
	l.held = true
	return nil
}
func (l *memoryLock) DelPushToSearchJob(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	l.held = false
	return nil
}

func post(id int64) *ms.Post { return &ms.Post{Model: &ms.Model{ID: id}, UserID: 1} }
func TestSingleAndFullSyncHaveIdenticalTextProjection(t *testing.T) {
	store := &memoryStore{posts: []*ms.Post{post(1)}, contents: []*ms.PostContent{
		{Model: &ms.Model{ID: 1}, PostID: 1, Content: "title", Type: ms.ContentTypeTitle},
		{Model: &ms.Model{ID: 2}, PostID: 1, Content: "image.jpg", Type: ms.ContentTypeImage},
		{Model: &ms.Model{ID: 3}, PostID: 1, Content: "body", Type: ms.ContentTypeText},
		{Model: &ms.Model{ID: 4}, PostID: 1, Content: "**markdown**", Type: ms.ContentTypeMarkdown},
	}}
	writer := &memoryWriter{}
	lock := &memoryLock{}
	index := searchindex.New(store, writer, lock)
	if err := index.Put(store.posts[0]); err != nil {
		t.Fatal(err)
	}
	if err := index.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(writer.docs) != 2 || !reflect.DeepEqual(writer.docs[0], writer.docs[1]) || writer.docs[0].Content != "title\nbody\n**markdown**\n" {
		t.Fatalf("documents = %+v", writer.docs)
	}
	if err := index.Delete(store.posts[0]); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(writer.deleted, []string{"1"}) || lock.held {
		t.Fatalf("delete = %v, held = %v", writer.deleted, lock.held)
	}
}

func TestSyncCoversEveryPageAndCanRunAgain(t *testing.T) {
	store := &memoryStore{}
	for i := int64(1); i <= 1001; i++ {
		store.posts = append(store.posts, post(i))
	}
	writer := &memoryWriter{}
	lock := &memoryLock{}
	index := searchindex.New(store, writer, lock)
	for run := 0; run < 2; run++ {
		if err := index.Sync(context.Background()); err != nil {
			t.Fatal(err)
		}
		if lock.held {
			t.Fatal("sync retained lock")
		}
	}
	if len(writer.docs) != 2002 || writer.docs[1000].Post.ID != 1001 || writer.docs[1001].Post.ID != 1 {
		t.Fatalf("sync missed a page: %d documents", len(writer.docs))
	}
}

func TestSyncReturnsFailuresAndReleasesAcquiredLock(t *testing.T) {
	failure := errors.New("unavailable")
	for _, tc := range []struct {
		name   string
		store  *memoryStore
		writer *memoryWriter
		lock   *memoryLock
	}{
		{"read", &memoryStore{err: failure}, &memoryWriter{}, &memoryLock{}},
		{"projection", &memoryStore{posts: []*ms.Post{post(1)}, mismatch: true}, &memoryWriter{}, &memoryLock{}},
		{"write", &memoryStore{posts: []*ms.Post{post(1)}}, &memoryWriter{err: failure}, &memoryLock{}},
		{"lock", &memoryStore{}, &memoryWriter{}, &memoryLock{err: failure}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			index := searchindex.New(tc.store, tc.writer, tc.lock)
			if err := index.Sync(context.Background()); err == nil {
				t.Fatal("failure swallowed")
			}
			if tc.lock.held {
				t.Fatal("failure retained lock")
			}
		})
	}
}

func TestSyncCancellationReleasesLock(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	store := &memoryStore{}
	for i := int64(1); i <= 1001; i++ {
		store.posts = append(store.posts, post(i))
	}
	lock := &memoryLock{}
	writer := &memoryWriter{afterWrite: cancel}
	index := searchindex.New(store, writer, lock)
	if err := index.Sync(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation = %v", err)
	}
	if lock.held {
		t.Fatal("canceled context prevented unlock")
	}
}
