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
	"github.com/golang-migrate/migrate/v4/database"
	"github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/sirupsen/logrus"
)

func Run() error {
	if !cfg.If("Migration") {
		logrus.Infoln("skip migrate because not add Migration feature in config.yaml")
		return nil
	}

	var (
		db        *sql.DB
		dbName    string
		dbDriver  database.Driver
		srcDriver source.Driver
		err, err2 error
	)

	if cfg.If("MySQL") {
		dbName = conf.MysqlSetting.DBName
		db, err = sql.Open("mysql", conf.MysqlSetting.Dsn()+"&multiStatements=true")
	} else if cfg.If("PostgreSQL") || cfg.If("Postgres") {
		dbName = (*conf.PostgresSetting)["DBName"]
		db, err = sql.Open("pgx", conf.PostgresSetting.Dsn())
	} else {
		dbName = conf.MysqlSetting.DBName
		db, err = sql.Open("mysql", conf.MysqlSetting.Dsn())
	}
	if err != nil {
		return fmt.Errorf("initial db for migration failed: %w", err)
	}

	migrationsTable := conf.DatabaseSetting.TablePrefix + "schema_migrations"
	if cfg.If("MySQL") {
		srcDriver, err = iofs.New(migration.Files, "mysql")
		dbDriver, err2 = mysql.WithInstance(db, &mysql.Config{MigrationsTable: migrationsTable})
	} else if cfg.If("PostgreSQL") || cfg.If("Postgres") {
		srcDriver, err = iofs.New(migration.Files, "postgres")
		dbDriver, err2 = postgres.WithInstance(db, &postgres.Config{MigrationsTable: migrationsTable})
	} else {
		srcDriver, err = iofs.New(migration.Files, "mysql")
		dbDriver, err2 = mysql.WithInstance(db, &mysql.Config{MigrationsTable: migrationsTable})
	}

	if err2 != nil {
		return fmt.Errorf("new database driver failed: %w", err2)
	}
	defer dbDriver.Close()
	if err != nil {
		return fmt.Errorf("new source driver failed: %w", err)
	}

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
