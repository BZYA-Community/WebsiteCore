//go:build migration

// Package testutil provides isolated PostgreSQL schemas for integration tests.
package testutil

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/BZYA-Community/WebsiteCore/scripts/migration"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

// Postgres never modifies the caller's existing tables. Cleanup drops only the
// randomly named schema created by this test. The DSN requires CREATE SCHEMA.
func Postgres(t *testing.T, version int) (*gorm.DB, *sql.DB) {
	t.Helper()
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	admin := stdlib.OpenDB(*config)
	buf := make([]byte, 8)
	if _, err = rand.Read(buf); err != nil {
		t.Fatal(err)
	}
	name := "identity_test_" + hex.EncodeToString(buf)
	if _, err = admin.ExecContext(context.Background(), "CREATE SCHEMA "+name); err != nil {
		t.Fatal(err)
	}
	config.RuntimeParams["search_path"] = name
	db := stdlib.OpenDB(*config)
	t.Cleanup(func() {
		db.Close()
		if _, err := admin.ExecContext(context.Background(), "DROP SCHEMA "+name+" CASCADE"); err != nil {
			t.Error(err)
		}
		admin.Close()
	})
	entries, err := migration.Files.ReadDir("postgres")
	if err != nil {
		t.Fatal(err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		file := entry.Name()
		if !strings.HasSuffix(file, ".up.sql") || file[:4] > fmt.Sprintf("%04d", version) {
			continue
		}
		Apply(t, db, file)
	}
	gdb, err := gorm.Open(postgres.New(postgres.Config{Conn: db}), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "p_", SingularTable: true},
		Logger:         logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	return gdb, db
}

func Apply(t *testing.T, db *sql.DB, file string) {
	t.Helper()
	data, err := migration.Files.ReadFile("postgres/" + file)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(context.Background(), string(data)); err != nil {
		t.Fatalf("%s: %v", file, err)
	}
}
