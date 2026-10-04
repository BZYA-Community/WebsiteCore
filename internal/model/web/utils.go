// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package web

import (
	"github.com/BZYA-Community/WebsiteCore/internal/application/media"
	"github.com/BZYA-Community/WebsiteCore/pkg/xerror"
	"github.com/alimy/mir/v5"
)

func fileCheck(uploadType string, size int64) mir.Error {
	if uploadType != "public/video" &&
		uploadType != "public/image" &&
		uploadType != "public/course-image" &&
		uploadType != "public/avatar" &&
		uploadType != "attachment" {
		return xerror.InvalidParams
	}
	limit := media.AttachmentLimit
	switch uploadType {
	case "public/video":
		limit = media.VideoLimit
	case "public/course-image":
		limit = media.CourseImageLimit
	}
	if size <= 0 || size > limit {
		return ErrFileInvalidSize
	}
	return nil
}
