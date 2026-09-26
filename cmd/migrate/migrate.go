// Copyright 2023 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package migrate

import (
	"fmt"
	"os"

	"github.com/BZYA-Community/WebsiteCore/cmd"
	"github.com/BZYA-Community/WebsiteCore/internal/conf"
	"github.com/BZYA-Community/WebsiteCore/internal/infra/migration"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func init() {
	migrateCmd := &cobra.Command{
		Use:   "migrate",
		Short: "migrate database data",
		Long:  "miegrate database data when paopao-ce upgrade",
		Run:   migrateRun,
	}
	cmd.Register(migrateCmd)
}

func migrateRun(_cmd *cobra.Command, _args []string) {
	// Force the Migration feature on regardless of config.yaml so this command
	// always migrates. The command must be built with the `migration` build tag,
	// otherwise migration.Run reports that the build lacks migration support and
	// migrateRun exits non-zero.
	if err := conf.Initial([]string{"Migration"}, false); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// This is a CLI command: surface migration logs on stderr instead of the
	// logger sinks configured for the server (e.g. LoggerFile), so the operator
	// actually sees progress and failures.
	logrus.SetOutput(os.Stderr)
	logrus.SetLevel(logrus.InfoLevel)

	if err := migration.Run(); err != nil {
		logrus.Error(err)
		os.Exit(1)
	}
}
