package jinzhu

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"os"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	migrationpostgres "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func newPostgresMigrationTest(t *testing.T) (*migrate.Migrate, *sql.Conn, *gorm.DB) {
	t.Helper()
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set; PostgreSQL migration integration test")
	}
	var token [6]byte
	if _, err := rand.Read(token[:]); err != nil {
		t.Fatal(err)
	}
	schemaName := "ut_migration_" + hex.EncodeToString(token[:])
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: schemaName + ".p_", SingularTable: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	pool, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pool.Close() })
	if err := db.Exec("CREATE SCHEMA " + schemaName).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Exec("DROP SCHEMA " + schemaName + " CASCADE").Error; err != nil {
			t.Errorf("drop migration test schema: %v", err)
		}
	})

	ctx := t.Context()
	conn, err := pool.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, "SET search_path TO "+schemaName); err != nil {
		_ = conn.Close()
		t.Fatal(err)
	}
	// Give golang-migrate a dedicated pgx connection like the application uses.
	driver, err := migrationpostgres.WithConnection(ctx, conn, &migrationpostgres.Config{MigrationsTable: "p_schema_migrations"})
	if err != nil {
		_ = conn.Close()
		t.Fatal(err)
	}
	runner, err := migrate.NewWithDatabaseInstance("file://../../../scripts/migration/postgres", "postgres", driver)
	if err != nil {
		_ = driver.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if sourceErr, dbErr := runner.Close(); sourceErr != nil || dbErr != nil {
			t.Errorf("close migration runner: source=%v database=%v", sourceErr, dbErr)
		}
	})
	return runner, conn, db
}
