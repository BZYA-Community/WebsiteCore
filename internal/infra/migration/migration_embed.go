// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

//go:build migration
// +build migration

package migration

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/BZYA-Community/WebsiteCore/internal/conf"
	"github.com/BZYA-Community/WebsiteCore/scripts/migration"
	"github.com/alimy/tryst/cfg"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/sirupsen/logrus"
)

func Run() error {
	if !cfg.If("Migration") {
		logrus.Infoln("skip migrate because not add Migration feature in config.yaml")
		return nil
	}

	dbName := (*conf.PostgresSetting)["DBName"]
	db, err := sql.Open("pgx", conf.PostgresSetting.Dsn())
	if err != nil {
		return fmt.Errorf("initial db for migration failed: %w", err)
	}
	defer db.Close()

	migrationsTable := conf.DatabaseSetting.TablePrefix + "schema_migrations"
	srcDriver, err := iofs.New(migration.Files, "postgres")
	if err != nil {
		return fmt.Errorf("new source driver failed: %w", err)
	}
	defer srcDriver.Close()
	dbDriver, err := postgres.WithInstance(db, &postgres.Config{MigrationsTable: migrationsTable})
	if err != nil {
		return fmt.Errorf("new database driver failed: %w", err)
	}
	defer dbDriver.Close()

	m, err := migrate.NewWithInstance("iofs", srcDriver, dbName, dbDriver)
	if err != nil {
		return fmt.Errorf("new migrate instance failed: %w", err)
	}

	if err = m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate up failed: %w", err)
	}
	logrus.Infoln("migrate up success")
	return nil
}
