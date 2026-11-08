// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package security

import (
	"fmt"
	"strings"

	"github.com/BZYA-Community/WebsiteCore/internal/conf"
	"github.com/BZYA-Community/WebsiteCore/internal/core"
)

type attachmentCheckServant struct {
	domain string
}

func (s *attachmentCheckServant) CheckAttachment(uri string) error {
	// 允许外部 http(s) 图片链接（#151）
	if strings.HasPrefix(uri, "http://") || strings.HasPrefix(uri, "https://") {
		for _, seg := range strings.Split(uri, "/") {
			if seg == ".." {
				return fmt.Errorf("附件路径非法")
			}
		}
		return nil
	}
	if strings.Index(uri, s.domain) != 0 {
		return fmt.Errorf("附件非本站资源")
	}
	// 路径越狱防护(#25): 仅查域名前缀拦不住 https://<domain>/../../custom/config.yaml,
	// 去掉前缀后的残余路径不允许含 ".." 段, 越狱键在 storage jailPath 再兜底拦截
	for _, seg := range strings.Split(strings.TrimPrefix(uri, s.domain), "/") {
		if seg == ".." {
			return fmt.Errorf("附件路径非法")
		}
	}
	return nil
}

func NewAttachmentCheckService() core.AttachmentCheckService {
	return &attachmentCheckServant{
		domain: conf.GetOssDomain(),
	}
}
