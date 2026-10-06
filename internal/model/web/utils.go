// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package web

import (
	"github.com/BZYA-Community/WebsiteCore/internal/conf"
	"github.com/BZYA-Community/WebsiteCore/pkg/xerror"
	"github.com/alimy/mir/v5"
)

func fileCheck(uploadType string, size int64) mir.Error {
	if uploadType != "public/video" &&
		uploadType != "public/image" &&
		uploadType != "public/avatar" &&
		uploadType != "attachment" {
		return xerror.InvalidParams
	}
	if size <= 0 || size > conf.UploadLimits().AttachmentMaxBytes {
		return ErrFileInvalidSize.WithDetails("Maximum attachment size is 15 MiB (or the lower configured limit)")
	}
	return nil
}
