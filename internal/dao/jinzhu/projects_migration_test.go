package jinzhu

import (
	"strings"
	"testing"
)

func TestProjectMigrationUpgradesMain35WithoutLosingURLs(t *testing.T) {
	runner, conn, _ := newPostgresMigrationTest(t)
	ctx := t.Context()
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := conn.ExecContext(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	if err := runner.Migrate(35); err != nil {
		t.Fatal(err)
	}
	longURL := "https://example.com/download?payload=" + strings.Repeat("a", 3000) + "&signature=must-survive-at-the-end"
	exec("INSERT INTO p_post(id,user_id,tags) VALUES (42,7,'项目')")
	exec("INSERT INTO p_post_content(post_id,user_id,type,content) VALUES (42,7,6,$1)", longURL)
	if err := runner.Migrate(36); err != nil {
		t.Fatal(err)
	}
	assertUpgrade := func() {
		t.Helper()
		version, dirty, err := runner.Version()
		if err != nil || dirty || version != 36 {
			t.Fatalf("version=%d dirty=%v err=%v", version, dirty, err)
		}
		var migrated, original string
		if err := conn.QueryRowContext(ctx, "SELECT url FROM p_project WHERE post_id=42").Scan(&migrated); err != nil {
			t.Fatal(err)
		}
		if err := conn.QueryRowContext(ctx, "SELECT content FROM p_post_content WHERE post_id=42 AND content=$1", longURL).Scan(&original); err != nil {
			t.Fatal(err)
		}
		if migrated != longURL || original != longURL {
			t.Fatal("signed legacy URL changed")
		}
		// Both column migrations must remain installed after project up/down/up.
		exec("SELECT admin_deleted_by, recreated_from_id FROM p_column LIMIT 0")
		exec("SELECT column_returned FROM p_message LIMIT 0")
	}
	assertUpgrade()
	if err := runner.Steps(-1); err != nil {
		t.Fatal(err)
	}
	version, dirty, err := runner.Version()
	if err != nil || dirty || version != 35 {
		t.Fatalf("rollback version=%d dirty=%v err=%v", version, dirty, err)
	}
	if err := runner.Migrate(36); err != nil {
		t.Fatal(err)
	}
	assertUpgrade()
}
