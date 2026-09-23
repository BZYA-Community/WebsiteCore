// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"github.com/BZYA-Community/WebsiteCore/cmd"
	_ "github.com/BZYA-Community/WebsiteCore/cmd/migrate"
	_ "github.com/BZYA-Community/WebsiteCore/cmd/serve"
)

func main() {
	cmd.Execute()
}
