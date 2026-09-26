// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package conf

import (
	"log"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

var (
	_gormdb   *gorm.DB
	_onceGorm sync.Once
)

func MustGormDB() *gorm.DB {
	_onceGorm.Do(func() {
		var err error
		if _gormdb, err = newGormDB(); err != nil {
			log.Fatalf("new gorm db failed: %s", err)
		}
	})
	return _gormdb
}

func closeGormDB() {
	db, err := _gormdb.DB()
	if err != nil {
		logrus.WithError(err).Error("get db from grom failed")
	}
	if err = db.Close(); err != nil {
		logrus.WithError(err).Error("close db failed")
	}
}

func newGormDB() (db *gorm.DB, err error) {
	newLogger := logger.New(
		logrus.StandardLogger(), // io writer（日志输出的目标，前缀和日志包含的内容）
		logger.Config{
			SlowThreshold:             time.Second,                // 慢 SQL 阈值
			LogLevel:                  DatabaseSetting.logLevel(), // 日志级别
			IgnoreRecordNotFoundError: true,                       // 忽略ErrRecordNotFound（记录未找到）错误
			Colorful:                  false,                      // 禁用彩色打印
		},
	)

	config := &gorm.Config{
		Logger: newLogger,
		NamingStrategy: schema.NamingStrategy{
			TablePrefix:   DatabaseSetting.TablePrefix,
			SingularTable: true,
		},
	}

	logrus.Debugln("use PostgreSQL as db")
	db, err = gorm.Open(postgres.Open(PostgresSetting.Dsn()), config)
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetConnMaxIdleTime(time.Hour)
	sqlDB.SetConnMaxLifetime(24 * time.Hour)
	sqlDB.SetMaxIdleConns(DatabaseSetting.MaxIdleConns)
	sqlDB.SetMaxOpenConns(DatabaseSetting.MaxOpenConns)
	return db, nil
}
