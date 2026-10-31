package jinzhu

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/BZYA-Community/WebsiteCore/internal/authz"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/jinzhu/dbr"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestProjectFollowAuthorizationTransactions(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set; PostgreSQL concurrent project follow integration test")
	}
	var token [6]byte
	if _, err := rand.Read(token[:]); err != nil {
		t.Fatal(err)
	}
	prefix := "ut_project_follow_" + hex.EncodeToString(token[:]) + "_"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{NamingStrategy: schema.NamingStrategy{TablePrefix: prefix, SingularTable: true}})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	models := []any{&dbr.User{}, &dbr.IdentityGroup{}, &dbr.IdentityGroupPermission{}, &dbr.UserIdentityGroup{}, &dbr.Post{}, &dbr.Project{}, &dbr.ProjectFollow{}}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(db.AutoMigrate(models...))
	defer func() {
		for i := len(models) - 1; i >= 0; i-- {
			db.Migrator().DropTable(models[i])
		}
	}()
	guest := &dbr.IdentityGroup{Key: "guest", Name: "Guest"}
	must(db.Create(guest).Error)
	for _, permission := range []string{authz.PostView, authz.CommunityInteract} {
		must(db.Create(&dbr.IdentityGroupPermission{GroupID: guest.ID, Permission: permission}).Error)
	}
	user := &ms.User{Model: &ms.Model{}, Username: "follower", Status: ms.UserStatusNormal}
	must(db.Create(user).Error)
	post := &ms.Post{Model: &ms.Model{}, UserID: user.ID + 1, Visibility: ms.PostVisitPublic, AuditStatus: ms.PostAuditApproved}
	must(db.Create(post).Error)
	project := &dbr.Project{Model: &ms.Model{}, PostID: post.ID, Title: "Project"}
	must(db.Create(project).Error)
	service := &projectSrv{db: db}
	assertCount := func(want int64) {
		t.Helper()
		var rows, count int64
		must(db.Model(&dbr.ProjectFollow{}).Where("project_id = ?", project.ID).Count(&rows).Error)
		must(db.Model(&dbr.Project{}).Where("id = ?", project.ID).Pluck("follow_count", &count).Error)
		if rows != want || count != want {
			t.Fatalf("follow rows=%d count=%d, want %d", rows, count, want)
		}
	}
	for _, revoke := range []string{"visibility", "permission", "deleted post"} {
		t.Run(revoke, func(t *testing.T) {
			// These snapshots model successful handler checks before authorization changes.
			must(dbr.LoadUserIdentities(db, user))
			if !user.HasPermission(authz.CommunityInteract) {
				t.Fatal("fixture cannot interact")
			}
			blocker := db.Begin()
			must(blocker.Error)
			defer blocker.Rollback()
			var pid int
			must(blocker.Raw("SELECT pg_backend_pid()").Scan(&pid).Error)
			must(blocker.Exec("SELECT id FROM "+prefix+"project WHERE id = ? FOR UPDATE", project.ID).Error)
			switch revoke {
			case "visibility":
				must(blocker.Model(&ms.Post{}).Where("id = ?", post.ID).Update("visibility", ms.PostVisitPrivate).Error)
			case "deleted post":
				must(blocker.Model(&ms.Post{}).Where("id = ?", post.ID).Update("is_del", 1).Error)
			case "permission":
				must(blocker.Exec("SELECT id FROM " + prefix + "identity_group WHERE key = 'guest' FOR UPDATE").Error)
				must(blocker.Where("group_id = ? AND permission = ?", guest.ID, authz.CommunityInteract).Delete(&dbr.IdentityGroupPermission{}).Error)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			done := make(chan error, 1)
			go func() {
				_, _, err := (&projectSrv{db: db.WithContext(ctx)}).SetProjectFollow(project.ID, user.ID, true)
				done <- err
			}()
			// Observe an actual PostgreSQL lock wait, not a timing-dependent sleep.
			deadline := time.Now().Add(5 * time.Second)
			for {
				var waiting bool
				must(db.Raw("SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE ? = ANY(pg_blocking_pids(pid)))", pid).Scan(&waiting).Error)
				if waiting {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("follow did not wait for the revocation transaction")
				}
				time.Sleep(10 * time.Millisecond)
			}
			must(blocker.Commit().Error)
			err := <-done
			if !errors.Is(err, authz.ErrDenied) && !errors.Is(err, gorm.ErrRecordNotFound) {
				t.Fatalf("follow after %s revocation: %v", revoke, err)
			}
			assertCount(0)
			must(db.Unscoped().Model(&ms.Post{}).Where("id = ?", post.ID).Updates(map[string]any{"visibility": ms.PostVisitPublic, "is_del": 0}).Error)
			if revoke == "permission" {
				must(db.Create(&dbr.IdentityGroupPermission{GroupID: guest.ID, Permission: authz.CommunityInteract}).Error)
			}
		})
	}
	t.Run("project wait retains authorization locks", func(t *testing.T) {
		blocker := db.Begin()
		must(blocker.Error)
		defer blocker.Rollback()
		var pid int
		must(blocker.Raw("SELECT pg_backend_pid()").Scan(&pid).Error)
		must(blocker.Exec("SELECT id FROM "+prefix+"project WHERE id = ? FOR UPDATE", project.ID).Error)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		done := make(chan error, 1)
		go func() {
			_, _, err := (&projectSrv{db: db.WithContext(ctx)}).SetProjectFollow(project.ID, user.ID, true)
			done <- err
		}()
		deadline := time.Now().Add(5 * time.Second)
		for {
			var waiting bool
			must(db.Raw("SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE ? = ANY(pg_blocking_pids(pid)))", pid).Scan(&waiting).Error)
			if waiting {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("follow did not wait for the project lock")
			}
			time.Sleep(10 * time.Millisecond)
		}
		// A revocation cannot commit while an authorized follow is queued on
		// the project row. It must serialize after that follow's commit.
		for _, query := range []string{
			"SELECT id FROM " + prefix + "identity_group WHERE key = 'guest' FOR UPDATE NOWAIT",
			"SELECT id FROM " + prefix + "post WHERE id = ? FOR UPDATE NOWAIT",
		} {
			var args []any
			if strings.Contains(query, "id = ?") {
				args = []any{post.ID}
			}
			err := db.Exec(query, args...).Error
			var state interface{ SQLState() string }
			if !errors.As(err, &state) || state.SQLState() != "55P03" {
				t.Fatalf("authorization row was not locked during project wait: %v", err)
			}
		}
		must(blocker.Commit().Error)
		must(<-done)
		assertCount(1)
		_, _, err := service.SetProjectFollow(project.ID, user.ID, false)
		must(err)
		assertCount(0)
	})
	// Duplicate concurrent requests must still produce one relationship and one count.
	for _, follow := range []bool{true, false} {
		var wg sync.WaitGroup
		failures := make(chan error, 8)
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, _, err := service.SetProjectFollow(project.ID, user.ID, follow)
				failures <- err
			}()
		}
		wg.Wait()
		close(failures)
		for err := range failures {
			must(err)
		}
		if follow {
			assertCount(1)
		} else {
			assertCount(0)
		}
	}
}
