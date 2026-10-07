// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package web

import (
	"image"
	"strings"
	"unicode/utf8"

	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/internal/model/web"
	"github.com/BZYA-Community/WebsiteCore/pkg/utils"
	"github.com/BZYA-Community/WebsiteCore/pkg/xerror"
	"github.com/gofrs/uuid/v5"
	"github.com/sirupsen/logrus"
)

// checkPassword 密码检查
func checkPassword(password string) error {
	// 检测用户是否合规
	if utf8.RuneCountInString(password) < 6 || utf8.RuneCountInString(password) > 16 {
		return web.ErrPasswordLengthLimit
	}
	return nil
}

// validPassword 检查密码是否一致(bcrypt, 内部恒定时间比较)
func validPassword(secret, password string) bool {
	return utils.ComparePassword(secret, password)
}

// encryptPasswordAndSalt 密码加密(bcrypt)并生成salt
// 注: salt仅用于JWT issuer绑定(改密码使旧token失效), 不参与密码哈希
func encryptPasswordAndSalt(password string) (string, string) {
	salt := uuid.Must(uuid.NewV4()).String()[:8]
	return utils.HashPassword(password), salt
}

// persistMediaContents 获取媒体内容并持久化
// Shared URLs may be referenced by other content. Failure does not authorize deletion.
func persistMediaContents(oss core.ObjectStorageService, contents []*web.PostContentItem) error {
	for _, item := range contents {
		switch item.Type {
		case ms.ContentTypeImage,
			ms.ContentTypeVideo,
			ms.ContentTypeAudio,
			ms.ContentTypeAttachment,
			ms.ContentTypeChargeAttachment:
			if err := oss.PersistObject(oss.ObjectKey(item.Content)); err != nil {
				logrus.Errorf("service.persistMediaContents failed: %s", err)
				return err
			}
		}
	}
	return nil
}

func fileCheck(uploadType string, size int64) error {
	if uploadType != "public/video" &&
		uploadType != "public/image" &&
		uploadType != "public/avatar" &&
		uploadType != "attachment" {
		return xerror.InvalidParams
	}
	if size > 1024*1024*100 {
		return web.ErrFileInvalidSize.WithDetails("最大允许100MB")
	}
	return nil
}

func getFileExt(s string) (string, error) {
	switch s {
	case "image/png":
		return ".png", nil
	case "image/jpg":
		return ".jpg", nil
	case "image/jpeg":
		return ".jpeg", nil
	case "image/gif":
		return ".gif", nil
	case "video/mp4":
		return ".mp4", nil
	case "video/quicktime":
		return ".mov", nil
	case "application/zip",
		"application/x-zip",
		"application/octet-stream",
		"application/x-zip-compressed":
		return ".zip", nil
	default:
		return "", web.ErrFileInvalidExt.WithDetails("仅允许 png/jpg/gif/mp4/mov/zip 类型")
	}
}

func generatePath(s string) string {
	n := len(s)
	if n <= 2 {
		return s
	}
	return generatePath(s[:n-2]) + "/" + s[n-2:]
}

func getImageSize(img image.Rectangle) (int, int) {
	b := img.Bounds()
	width := b.Max.X
	height := b.Max.Y
	return width, height
}

func tagsFrom(originTags []string) []string {
	tags := make([]string, 0, len(originTags))
	for _, tag := range originTags {
		// TODO: 优化tag有效性检测
		if tag = strings.TrimSpace(tag); len(tag) > 0 {
			tags = append(tags, tag)
		}
	}
	return tags
}

// checkPermision checks ownership or explicit content-management permission.
func checkPermision(user *ms.User, targetUserId int64) error {
	if user == nil || (user.ID != targetUserId && !user.HasPermission("content.manage")) {
		return web.ErrNoPermission
	}
	return nil
}

// Pending parents accept replies from their author or an authorized moderator;
// rejected parents never accept replies.
func canReplyToComment(user *ms.User, authorID int64, status ms.PostAuditT) bool {
	return user != nil && user.HasPermission("comment.create") &&
		(status == ms.PostAuditApproved || status == ms.PostAuditPending &&
			(user.ID == authorID || user.HasPermission("audit.view_all")))
}
