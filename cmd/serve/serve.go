// Copyright 2023 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package serve

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/alimy/tryst/cfg"
	"github.com/fatih/color"
	"github.com/getsentry/sentry-go"
	"github.com/BZYA-Community/WebsiteCore/cmd"
	"github.com/BZYA-Community/WebsiteCore/internal"
	"github.com/BZYA-Community/WebsiteCore/internal/conf"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/cache"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/jinzhu/dbr"
	"github.com/BZYA-Community/WebsiteCore/internal/service"
	"github.com/BZYA-Community/WebsiteCore/internal/sitesetting"
	"github.com/BZYA-Community/WebsiteCore/pkg/debug"
	"github.com/BZYA-Community/WebsiteCore/pkg/utils"
	"github.com/BZYA-Community/WebsiteCore/pkg/version"
	"github.com/gofrs/uuid/v5"
	"github.com/sirupsen/logrus"
	"github.com/sourcegraph/conc"
	"github.com/spf13/cobra"
	"go.uber.org/automaxprocs/maxprocs"
	"gorm.io/gorm"
)

var (
	noDefaultFeatures bool
	features          []string
)

func init() {
	serveCmd := &cobra.Command{
		Use:   "serve",
		Short: "start paopao-ce server",
		Long:  "start paopao-ce server",
		Run:   serveRun,
	}

	serveCmd.Flags().BoolVar(&noDefaultFeatures, "no-default-features", false, "whether not use default features")
	serveCmd.Flags().StringSliceVarP(&features, "features", "f", []string{}, "use special features")

	cmd.Register(serveCmd)
}

func deferFn() {
	if cfg.If("Sentry") {
		// Flush buffered events before the program terminates.
		sentry.Flush(2 * time.Second)
	}
	conf.CloseDB()
}

// _defaultOperatorAvatar 运维账号自动创建时使用的默认头像(与注册流程同源)
const _defaultOperatorAvatar = "https://paopao-demo.vercel.app/avatar/default/zoe.png"

// ensureOperatorAccount 幂等确保配置的运维账号可用:
// 账号不存在时按配置创建；存在时密码以配置为准(不一致则重置)；始终确保 operator 角色与 is_admin
func ensureOperatorAccount() {
	op := conf.OperatorSetting
	if op == nil || op.Username == "" {
		return
	}
	db := conf.MustGormDB()
	user := &dbr.User{}
	err := db.Where("username = ?", op.Username).First(user).Error
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			logrus.Errorf("query operator account[%s] failure by err: %v", op.Username, err)
			return
		}
		// 账号不存在: 按配置创建运维账号
		if op.Password == "" {
			logrus.Warnf("operator account[%s] not found and no password configured, skip create", op.Username)
			return
		}
		if utf8.RuneCountInString(op.Password) < 6 || utf8.RuneCountInString(op.Password) > 16 {
			logrus.Errorf("operator account[%s] password invalid(need 6-16 chars), skip create", op.Username)
			return
		}
		salt := uuid.Must(uuid.NewV4()).String()[:8]
		user = &dbr.User{
			Model:    &dbr.Model{},
			Nickname: op.Username,
			Username: op.Username,
			Password: utils.EncodeMD5(utils.EncodeMD5(op.Password) + salt),
			Salt:     salt,
			Avatar:   _defaultOperatorAvatar,
			Status:   ms.UserStatusNormal,
			IsAdmin:  true,
			Roles:    ms.RoleOperator,
		}
		if _, err := user.Create(db); err != nil {
			logrus.Errorf("create operator account[%s] failure by err: %v", op.Username, err)
			return
		}
		logrus.Infof("create operator account[%s] success", op.Username)
		return
	}

	// 账号已存在: 确保角色/密码与配置一致
	updates := map[string]any{}
	oldRoles := user.Roles
	if user.AddRole(ms.RoleOperator) {
		updates["roles"] = user.Roles
	}
	user.SyncIsAdmin()
	if !user.IsAdmin {
		updates["is_admin"] = true
		user.IsAdmin = true
	}
	resetPassword := false
	if op.Password != "" {
		// 密码以配置为准，不一致则重置
		expected := utils.EncodeMD5(utils.EncodeMD5(op.Password) + user.Salt)
		if expected != user.Password {
			if utf8.RuneCountInString(op.Password) < 6 || utf8.RuneCountInString(op.Password) > 16 {
				logrus.Errorf("operator account[%s] password invalid(need 6-16 chars), skip reset", op.Username)
			} else {
				salt := uuid.Must(uuid.NewV4()).String()[:8]
				updates["password"] = utils.EncodeMD5(utils.EncodeMD5(op.Password) + salt)
				updates["salt"] = salt
				resetPassword = true
			}
		}
	}
	if len(updates) == 0 {
		return
	}
	if err := db.Model(user).Updates(updates).Error; err != nil {
		logrus.Errorf("ensure operator account[%s] failure by err: %v", op.Username, err)
		return
	}
	logrus.Infof("ensure operator account[%s]: roles %q -> %q, resetPassword=%v", op.Username, oldRoles, user.Roles, resetPassword)
	// 过期该用户缓存，避免旧 gob 数据残留
	ac := cache.NewAppCache()
	ac.Delete(conf.KeyUserInfoById.Get(user.ID),
		conf.KeyUserInfoByName.Get(user.Username),
		conf.KeyUserProfileByName.Get(user.Username))
}

func serveRun(_cmd *cobra.Command, _args []string) {
	utils.PrintHelloBanner(version.VersionInfo())

	// set maxprocs automatic
	maxprocs.Set(maxprocs.Logger(log.Printf))

	// initial configure
	conf.Initial(features, noDefaultFeatures)
	if cfg.If("loggerOtlp") {
		shutdownFn, _ := conf.InitTelemetry()
		defer shutdownFn()
	}
	internal.Initial()
	sitesetting.Bootstrap(_cmd.Context(), conf.MustGormDB())
	ensureOperatorAccount()
	ss := service.MustInitService()
	if len(ss) < 1 {
		fmt.Fprintln(color.Output, "no service need start so just exit")
		return
	}

	// do defer function
	defer deferFn()

	// start pyroscope if need
	debug.StartPyroscope()

	// start services
	wg := conc.NewWaitGroup()
	fmt.Fprintf(color.Output, "\nstarting run service...\n\n")
	service.Start(wg)

	// graceful stop services
	wg.Go(func() {
		quit := make(chan os.Signal, 1)
		// kill (no param) default send syscall.SIGTERM
		// kill -2 is syscall.SIGINT
		// kill -9 is syscall.SIGKILL but can't be catch, so don't need add it
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		fmt.Fprintf(color.Output, "\nshutting down server...\n\n")
		service.Stop()
	})
	wg.Wait()
}
