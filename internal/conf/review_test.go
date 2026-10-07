package conf

import "testing"

func TestReviewSettingsDefaultsAndBounds(t *testing.T) {
	defaults := &AuditConf{}
	if err := defaults.NormalizeReviewSettings(); err != nil || defaults.DeadlineHours != 48 || defaults.AssignmentIntervalSeconds != 60 {
		t.Fatalf("defaults: %+v, %v", defaults, err)
	}
	for _, setting := range []AuditConf{{DeadlineHours: -1}, {DeadlineHours: 721}, {AssignmentIntervalSeconds: 1}, {AssignmentIntervalSeconds: 3601}} {
		if setting.NormalizeReviewSettings() == nil {
			t.Fatalf("accepted invalid review config: %+v", setting)
		}
	}
}
