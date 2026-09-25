// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package internal

import (
	"github.com/BZYA-Community/WebsiteCore/internal/infra/events"
	"github.com/BZYA-Community/WebsiteCore/internal/infra/metrics"
	"github.com/BZYA-Community/WebsiteCore/internal/infra/migration"
	"github.com/sirupsen/logrus"
)

func Initial() {
	// migrate database if needed
	if err := migration.Run(); err != nil {
		logrus.Errorf("run database migration failed: %v", err)
	}
	// event manager system initialize
	events.Initial()
	// metric manager system initialize
	metrics.Initial()
}
