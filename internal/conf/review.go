package conf

import (
	"fmt"
	"time"
)

func (c *AuditConf) NormalizeReviewSettings() error {
	if c.DeadlineHours == 0 {
		c.DeadlineHours = 48
	}
	if c.AssignmentIntervalSeconds == 0 {
		c.AssignmentIntervalSeconds = 60
	}
	if c.DeadlineHours < 1 || c.DeadlineHours > 720 {
		return fmt.Errorf("Audit.DeadlineHours must be between 1 and 720")
	}
	if c.AssignmentIntervalSeconds < 5 || c.AssignmentIntervalSeconds > 3600 {
		return fmt.Errorf("Audit.AssignmentIntervalSeconds must be between 5 and 3600")
	}
	return nil
}

func ReviewDeadline() time.Duration {
	if AuditSetting == nil || AuditSetting.DeadlineHours == 0 {
		return 48 * time.Hour
	}
	return time.Duration(AuditSetting.DeadlineHours) * time.Hour
}
