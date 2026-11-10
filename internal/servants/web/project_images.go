package web

import (
	"net/url"
	"strings"
	"unicode"

	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/internal/model/web"
)

// External URLs are supported only for project images. They are client-side
// references, never fetched or passed to object storage by the server.
func projectImages(check core.AttachmentCheckService, oss core.ObjectStorageService, urls []string) ([]*ms.ProjectImage, []*web.PostContentItem, error) {
	if len(urls) < 1 || len(urls) > 6 {
		return nil, nil, web.ErrProjectInputInvalid.WithDetails("项目概述图必须为1至6张")
	}
	base, err := url.ParseRequestURI(oss.ObjectURL(""))
	if err != nil || base.Host == "" {
		return nil, nil, web.ErrProjectInputInvalid.WithDetails("项目图片存储配置无效")
	}
	images := make([]*ms.ProjectImage, 0, len(urls))
	local := make([]*web.PostContentItem, 0, len(urls))
	for index, raw := range urls {
		parsed, err := url.ParseRequestURI(raw)
		if err != nil || len(raw) > 8192 || parsed.Hostname() == "" || parsed.User != nil ||
			(parsed.Scheme != "http" && parsed.Scheme != "https") ||
			strings.ContainsAny(raw, "\\") || strings.IndexFunc(raw, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) >= 0 {
			return nil, nil, web.ErrProjectInputInvalid.WithDetails("项目概述图必须是有效的 http(s) 地址")
		}
		for _, segment := range strings.Split(parsed.Path, "/") {
			if segment == ".." || strings.Contains(segment, "\\") {
				return nil, nil, web.ErrProjectInputInvalid.WithDetails("项目概述图路径不合法")
			}
		}
		// Match the parsed storage authority, not a substring in the path or
		// query. Same-host URLs still have to satisfy the storage path policy.
		if strings.EqualFold(parsed.Host, base.Host) {
			if err := check.CheckAttachment(raw); err != nil {
				return nil, nil, web.ErrProjectInputInvalid.WithDetails("项目概述图不合法")
			}
			local = append(local, &web.PostContentItem{Content: raw, Type: ms.ContentTypeImage, Sort: int64(index)})
		}
		images = append(images, &ms.ProjectImage{URL: raw, Sort: int64(index)})
	}
	return images, local, nil
}
