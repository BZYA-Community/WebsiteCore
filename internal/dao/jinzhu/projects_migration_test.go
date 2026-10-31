package jinzhu

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"os"
	"strings"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	migrationpostgres "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func TestProjectMigrationUpgradesMain35WithoutLosingURLs(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set; PostgreSQL project upgrade integration test")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	var token [6]byte
	if _, err := rand.Read(token[:]); err != nil {
		t.Fatal(err)
	}
	schema := "ut_project_upgrade_" + hex.EncodeToString(token[:])
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := conn.ExecContext(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec("CREATE SCHEMA " + schema)
	defer db.ExecContext(ctx, "DROP SCHEMA "+schema+" CASCADE")
	exec("SET search_path TO " + schema)
	driver, err := migrationpostgres.WithConnection(ctx, conn, &migrationpostgres.Config{SchemaName: schema})
	if err != nil {
		t.Fatal(err)
	}
	runner, err := migrate.NewWithDatabaseInstance("file://../../../scripts/migration/postgres", "postgres", driver)
	if err != nil {
		t.Fatal(err)
	}
	defer runner.Close()
	// Use the real migration engine, so duplicate versions and skipped upgrades fail.
	if err := runner.Migrate(35); err != nil {
		t.Fatal(err)
	}
	longURL := "https://example.com/download?payload=" + strings.Repeat("a", 3000) + "&signature=must-survive-at-the-end"
	exec("INSERT INTO p_post(id,user_id,tags) VALUES (42,7,'项目')")
	exec("INSERT INTO p_post_content(post_id,user_id,type,content) VALUES (42,7,6,$1)", longURL)
	if err := runner.Up(); err != nil {
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
	if err := runner.Up(); err != nil {
		t.Fatal(err)
	}
	assertUpgrade()
}
