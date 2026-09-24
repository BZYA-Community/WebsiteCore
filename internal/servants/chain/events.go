// Copyright 2023 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package chain

import (
	"github.com/BZYA-Community/WebsiteCore/internal/infra/events"
	"github.com/BZYA-Community/WebsiteCore/internal/model/web"
	"github.com/alimy/tryst/event"
	"github.com/sirupsen/logrus"
)

type AuditHookEvent struct {
	event.UnimplementedEvent
	ami *web.AuditMetaInfo
}

func (e *AuditHookEvent) Name() string {
	return "AuditHookEvent"
}

func (e *AuditHookEvent) Action() error {
	// TODO: just log event now， will add real logic in future.
	logrus.Debugf("auditHook event action style[%s] id[%d]", e.ami.Style, e.ami.Id)
	return nil
}

func OnAudiotHookEvent(ami *web.AuditMetaInfo) {
	if ami != nil {
		events.OnEvent(&AuditHookEvent{
			ami: ami,
		})
	}
}
