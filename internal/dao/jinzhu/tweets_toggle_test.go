package jinzhu

import (
	"errors"
	"slices"
	"sync"
	"testing"

	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/jinzhu/dbr"
	"gorm.io/gorm"
)

type postPinEvents struct {
	mu       sync.Mutex
	statuses []int
}

func (events *postPinEvents) SendAction(action core.IdxAct, post *dbr.Post) {
	if action == core.IdxActStickPost {
		events.mu.Lock()
		events.statuses = append(events.statuses, post.IsTop)
		events.mu.Unlock()
	}
}

func TestConcurrentPostToggles(t *testing.T) {
	runner, _, db := newPostgresMigrationTest(t)
	if err := runner.Up(); err != nil {
		t.Fatal(err)
	}

	events := &postPinEvents{}
	posts := newTweetService(db)
	manage := newTweetManageService(db, events)
	for _, test := range []struct {
		name   string
		toggle func(*dbr.Post) (int, error)
		status func(*dbr.Post) int
	}{
		{
			name:   "pin",
			toggle: manage.StickPost,
			status: func(post *dbr.Post) int { return post.IsTop },
		},
		{
			name:   "lock",
			toggle: manage.LockPost,
			status: func(post *dbr.Post) int { return post.IsLock },
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			post := &dbr.Post{Model: &dbr.Model{}, UserID: 42}
			if err := db.Create(post).Error; err != nil {
				t.Fatal(err)
			}
			// Both requests read the same state before either can toggle it.
			var snapshots [2]*dbr.Post
			for i := range snapshots {
				snapshot, err := posts.GetPostByID(post.ID)
				if err != nil {
					t.Fatal(err)
				}
				snapshots[i] = snapshot
			}

			type result struct {
				status int
				err    error
			}
			start := make(chan struct{})
			results := make(chan result, len(snapshots))
			for _, snapshot := range snapshots {
				go func() {
					<-start
					status, err := test.toggle(snapshot)
					results <- result{status: status, err: err}
				}()
			}
			close(start)
			var statuses []int
			for range snapshots {
				result := <-results
				if result.err != nil {
					t.Errorf("toggle failed: %v", result.err)
				}
				statuses = append(statuses, result.status)
			}
			slices.Sort(statuses)
			stored, err := posts.GetPostByID(post.ID)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(statuses, []int{0, 1}) || test.status(stored) != 0 {
				t.Errorf("two toggles returned %v and stored %d, want [0 1] and 0", statuses, test.status(stored))
			}
			if err := stored.Delete(db); err != nil {
				t.Fatal(err)
			}
			if _, err := test.toggle(snapshots[0]); !errors.Is(err, gorm.ErrRecordNotFound) {
				t.Errorf("toggle after deletion returned error %v, want %v", err, gorm.ErrRecordNotFound)
			}
		})
	}

	slices.Sort(events.statuses)
	if !slices.Equal(events.statuses, []int{0, 1}) {
		t.Errorf("pin cache events carried %v, want [0 1]", events.statuses)
	}
}
