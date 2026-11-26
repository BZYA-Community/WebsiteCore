package jinzhu

import (
	"context"
	"database/sql"
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/BZYA-Community/WebsiteCore/internal/authz"
	"github.com/BZYA-Community/WebsiteCore/internal/conf"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/jinzhu/dbr"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newReviewTestDatabase(t *testing.T) (*sql.Conn, *gorm.DB) {
	t.Helper()
	runner, conn, db := newPostgresMigrationTest(t)
	requireReviewTest(t, runner.Up())
	return conn, db
}

func requireReviewTest(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func execReviewTestSQL(t *testing.T, conn *sql.Conn, query string, args ...any) {
	t.Helper()
	_, err := conn.ExecContext(t.Context(), query, args...)
	requireReviewTest(t, err)
}

type queryTrace struct {
	logger.Interface
	trace func(string, int64)
}

func (l queryTrace) Trace(ctx context.Context, begin time.Time, result func() (string, int64), err error) {
	query, rows := result()
	l.trace(query, rows)
	l.Interface.Trace(ctx, begin, result, err)
}

func TestReviewSubmissionLooksUpOneReviewer(t *testing.T) {
	conn, db := newReviewTestDatabase(t)
	execReviewTestSQL(t, conn, `
		INSERT INTO p_user (id, username) SELECT n, 'user_' || n FROM generate_series(1, 201) AS n;
		INSERT INTO p_user_identity_group (user_id, group_id)
		SELECT 201, id FROM p_identity_group WHERE key = 'reviewer';
		INSERT INTO p_identity_group_permission (group_id, permission)
		SELECT id, 'post.create' FROM p_identity_group WHERE key = 'guest';`)
	oldAudit := conf.AuditSetting
	conf.AuditSetting = &conf.AuditConf{Enabled: true, DeadlineHours: 48}
	t.Cleanup(func() { conf.AuditSetting = oldAudit })

	// Check only the assignment reads that follow the submission event.
	var submitted bool
	var lookupQueries int
	var lookupRows int64
	trace := queryTrace{Interface: db.Logger, trace: func(query string, rows int64) {
		if submitted && strings.HasPrefix(query, "SELECT ") {
			lookupQueries++
			lookupRows += rows
		}
		if strings.HasPrefix(query, "INSERT INTO ") && strings.Contains(query, `"p_review_task_event"`) && strings.Contains(query, "'submitted'") {
			submitted = true
		}
	}}
	s := &reviewSrv{db: db.Session(&gorm.Session{Logger: trace})}
	post, err := s.CreatePostWithContents(&ms.Post{UserID: 1, Visibility: ms.PostVisitPublic}, nil)
	requireReviewTest(t, err)
	if lookupQueries != 1 || lookupRows != 1 {
		t.Fatalf("reviewer lookup returned %d rows in %d queries, want 1 row in 1 query", lookupRows, lookupQueries)
	}
	var task ms.ReviewTask
	requireReviewTest(t, db.Where("kind = ? AND target_id = ?", ms.ReviewPost, post.ID).First(&task).Error)
	if task.AssigneeID != 201 {
		t.Fatalf("assigned reviewer %d, want 201", task.AssigneeID)
	}
}

func TestReviewSweepReusesReviewerPool(t *testing.T) {
	conn, db := newReviewTestDatabase(t)
	execReviewTestSQL(t, conn, `
		INSERT INTO p_user (id, username) VALUES (1, 'reviewer'), (2, 'author');
		INSERT INTO p_user_identity_group (user_id, group_id)
		SELECT 1, id FROM p_identity_group WHERE key = 'reviewer';`)
	deadline := 48 * time.Hour
	requireReviewTest(t, db.Transaction(func(tx *gorm.DB) error {
		if err := reviewPolicyLock(tx); err != nil {
			return err
		}
		for _, authorID := range []int64{1, 1, 2, 2, 2} {
			post := &ms.Post{Model: &ms.Model{}, UserID: authorID, AuditStatus: ms.PostAuditPending, Visibility: ms.PostVisitPublic}
			if err := tx.Create(post).Error; err != nil {
				return err
			}
			if err := enqueueReviewTx(tx, ms.ReviewPost, post.ID, deadline); err != nil {
				return err
			}
		}
		return tx.Model(&ms.ReviewTask{}).Where("assignee_id = ?", 1).Update("deadline_on", time.Now().Unix()-1).Error
	}))
	var selfAuthoredTask ms.ReviewTask
	requireReviewTest(t, db.Where("author_id = ?", 1).First(&selfAuthoredTask).Error)
	var lookupQueries int
	trace := queryTrace{Interface: db.Logger, trace: func(query string, _ int64) {
		if strings.HasPrefix(query, "SELECT ") && strings.Contains(query, `"p_identity_group_permission"`) {
			lookupQueries++
		}
	}}
	s := &reviewSrv{db: db.Session(&gorm.Session{Logger: trace})}
	for sweep := 1; sweep <= 2; sweep++ {
		if sweep == 2 {
			// A superseded task needs a new revision and shares the same pool.
			requireReviewTest(t, db.Model(&selfAuthoredTask).Update("snapshot", "old").Error)
		}
		lookupQueries = 0
		requireReviewTest(t, s.ReconcileReviewTasks(deadline))
		if lookupQueries != 1 {
			t.Errorf("sweep %d used %d eligibility queries, want 1", sweep, lookupQueries)
		}
		var overdueOrSelfAssigned int64
		requireReviewTest(t, db.Model(&ms.ReviewTask{}).Where("assignee_id <> 0").Count(&overdueOrSelfAssigned).Error)
		if overdueOrSelfAssigned != 0 {
			t.Fatalf("sweep %d left %d tasks assigned to an excluded reviewer", sweep, overdueOrSelfAssigned)
		}
	}
	var timeouts int64
	requireReviewTest(t, db.Model(&ms.ReviewTaskEvent{}).Where("event = ?", "timeout").Count(&timeouts).Error)
	if timeouts != 3 {
		t.Fatalf("recorded %d timeout events, want 3", timeouts)
	}
	requireReviewTest(t, db.First(&selfAuthoredTask, selfAuthoredTask.ID).Error)
	if selfAuthoredTask.Revision != 2 {
		t.Fatalf("superseded task revision %d, want 2", selfAuthoredTask.Revision)
	}

	execReviewTestSQL(t, conn, `
		INSERT INTO p_user (id, username) VALUES (3, 'replacement');
		INSERT INTO p_user_identity_group (user_id, group_id)
		SELECT 3, id FROM p_identity_group WHERE key = 'reviewer';`)
	lookupQueries = 0
	requireReviewTest(t, s.ReconcileReviewTasks(deadline))
	var reassigned int64
	requireReviewTest(t, db.Model(&ms.ReviewTask{}).Where("assignee_id = ?", 3).Count(&reassigned).Error)
	if lookupQueries != 1 || reassigned != 5 {
		t.Fatalf("sweep with a new reviewer used %d eligibility queries and assigned %d tasks, want 1 query and 5 tasks", lookupQueries, reassigned)
	}
}

type reviewPlanNode struct {
	Relation         string           `json:"Relation Name"`
	Rows             float64          `json:"Actual Rows"`
	RemovedByFilter  float64          `json:"Rows Removed by Filter"`
	RemovedByRecheck float64          `json:"Rows Removed by Index Recheck"`
	Loops            float64          `json:"Actual Loops"`
	Children         []reviewPlanNode `json:"Plans"`
}

func (n reviewPlanNode) userRowsVisited() float64 {
	var rows float64
	if n.Relation == "p_user" {
		rows = (n.Rows + n.RemovedByFilter + n.RemovedByRecheck) * n.Loops
	}
	for _, child := range n.Children {
		rows += child.userRowsVisited()
	}
	return rows
}

func TestMemberReviewAssignmentScansFewUsers(t *testing.T) {
	conn, db := newReviewTestDatabase(t)
	execReviewTestSQL(t, conn, `
		INSERT INTO p_user (id, username) SELECT n, 'user_' || n FROM generate_series(1, 50000) AS n;
		UPDATE p_user SET email = 'member@example.test', phone = '5550000' WHERE id = 50000;
		INSERT INTO p_identity_group_permission (group_id, permission)
		SELECT id, 'content.review' FROM p_identity_group WHERE key = 'member';
		ANALYZE p_user;`)
	oldVerification := conf.AccountVerifySetting
	t.Cleanup(func() { conf.AccountVerifySetting = oldVerification })
	for _, mode := range []string{"email", "phone"} {
		t.Run(mode, func(t *testing.T) {
			conf.AccountVerifySetting = &conf.AccountVerifyConf{Mode: mode}
			var ids []int64
			lookup := dbr.EligibleReviewers(db).Where("id <> ? AND id <> ?", 1, 0).Order("random()").Limit(1)
			stmt := lookup.Session(&gorm.Session{DryRun: true}).Pluck("id", &ids).Statement
			var data []byte
			requireReviewTest(t, conn.QueryRowContext(t.Context(), "EXPLAIN (ANALYZE, FORMAT JSON) "+stmt.SQL.String(), stmt.Vars...).Scan(&data))
			var output []struct{ Plan reviewPlanNode }
			requireReviewTest(t, json.Unmarshal(data, &output))
			if len(output) != 1 || output[0].Plan.Rows != 1 {
				t.Fatalf("expected a one-row lookup, got %s", data)
			}
			visited := output[0].Plan.userRowsVisited()
			t.Logf("reviewer lookup visited %.0f user rows", visited)
			// The fixture has one eligible member. Allow a few extra rows
			// without depending on an exact node type or planner cost.
			if visited < 1 || visited > 10 {
				t.Errorf("reviewer lookup visited %.0f user rows, want 1–10", visited)
			}
			requireReviewTest(t, lookup.Pluck("id", &ids).Error)
			if !slices.Equal(ids, []int64{50000}) {
				t.Fatalf("assigned IDs %v, want [50000]", ids)
			}
		})
	}
}

func TestEligibleReviewersMatchesIdentityPolicy(t *testing.T) {
	conn, db := newReviewTestDatabase(t)
	execReviewTestSQL(t, conn, `
		INSERT INTO p_user (id, username, email, phone, status, is_del, is_operator) VALUES
			(1, 'unverified', '', '', 1, 0, false),
			(2, 'email_only', 'email@example.test', '', 1, 0, false),
			(3, 'phone_only', '', '5550000', 1, 0, false),
			(4, 'explicit_grants', '', '', 1, 0, false),
			(5, 'operator', '', '', 1, 0, true),
			(6, 'suspended_reviewer', '', '', 2, 0, false),
			(7, 'deleted_operator', '', '', 1, 1, true);
		INSERT INTO p_user_identity_group (user_id, group_id) SELECT 1, id FROM p_identity_group WHERE key = 'member';
		INSERT INTO p_user_identity_group (user_id, group_id) SELECT u, id FROM p_identity_group CROSS JOIN (VALUES (2), (3)) AS users(u) WHERE key = 'guest';
		INSERT INTO p_user_identity_group (user_id, group_id) SELECT 4, id FROM p_identity_group WHERE key IN ('admin', 'reviewer');
		INSERT INTO p_user_identity_group (user_id, group_id) SELECT 6, id FROM p_identity_group WHERE key = 'reviewer';`)
	oldVerification := conf.AccountVerifySetting
	t.Cleanup(func() { conf.AccountVerifySetting = oldVerification })
	for _, grant := range []string{"neither", "guest", "member", "both"} {
		t.Run(grant, func(t *testing.T) {
			execReviewTestSQL(t, conn, `
				DELETE FROM p_identity_group_permission WHERE permission = 'content.review'
				AND group_id IN (SELECT id FROM p_identity_group WHERE key IN ('guest', 'member'));`)
			execReviewTestSQL(t, conn, `
				INSERT INTO p_identity_group_permission (group_id, permission)
				SELECT id, 'content.review' FROM p_identity_group
				WHERE key = $1 OR ($1 = 'both' AND key IN ('guest', 'member'));`, grant)
			for _, mode := range []string{"email", "phone"} {
				t.Run(mode, func(t *testing.T) {
					conf.AccountVerifySetting = &conf.AccountVerifyConf{Mode: mode}
					var users []*dbr.User
					requireReviewTest(t, db.Order("id").Find(&users).Error)
					requireReviewTest(t, dbr.LoadUserIdentities(db, users...))
					var want, got []int64
					for _, user := range users {
						if user.HasPermission(authz.ContentReview) {
							want = append(want, user.ID)
						}
					}
					requireReviewTest(t, dbr.EligibleReviewers(db).Order("id").Pluck("id", &got).Error)
					if !slices.Equal(got, want) {
						t.Fatalf("SQL eligible reviewers %v, want %v from Go policy", got, want)
					}
				})
			}
		})
	}
}
