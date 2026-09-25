// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

//go:build !migration
// +build !migration

package migration

import (
	"fmt"

	"github.com/alimy/tryst/cfg"
)

func Run() error {
	if cfg.If("Migration") {
		return fmt.Errorf("migration feature requested but this build lacks the `migration` build tag")
	}
	return nil
}
