// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package security

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/BZYA-Community/WebsiteCore/internal/conf"
	"github.com/BZYA-Community/WebsiteCore/internal/core"
)

type attachmentCheckServant struct {
	domain string
}

func (s *attachmentCheckServant) CheckAttachment(uri string) error {
	// Keep ordinary attachments on our configured storage origin and path.
	// The project image endpoint handles external image references separately.
	base, err := url.ParseRequestURI(s.domain)
	if err != nil || base.Host == "" || s.domain == "" || !strings.HasSuffix(s.domain, "/") || !strings.HasPrefix(uri, s.domain) {
		return fmt.Errorf("附件非本站资源")
	}
	parsed, err := url.ParseRequestURI(uri)
	if err != nil || parsed.Scheme != base.Scheme || parsed.Host != base.Host || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || strings.ContainsAny(uri, "\\?#") {
		return fmt.Errorf("附件路径非法")
	}
	// URL.Path is decoded, so encoded traversal and separators are checked too.
	if strings.Contains(parsed.Path, "\\") {
		return fmt.Errorf("附件路径非法")
	}
	for _, segment := range strings.Split(parsed.Path, "/") {
		if segment == ".." {
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
