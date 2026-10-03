// Copyright 2026 BZYA Community. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package web

import "testing"

func TestIsAllowedEmailDomain(t *testing.T) {
	allowed := []string{"bza.edu.cn", "Example.EDU."}
	tests := []struct {
		email string
		want  bool
	}{
		{"student@bza.edu.cn", true},
		{"student@campus.bza.edu.cn", true},
		{"student@example.edu", true},
		{"student@temporary.example", false},
		{"student@evilbza.edu.cn", false},
		{"invalid", false},
	}
	for _, tt := range tests {
		if got := isAllowedEmailDomain(tt.email, allowed); got != tt.want {
			t.Errorf("isAllowedEmailDomain(%q) = %v, want %v", tt.email, got, tt.want)
		}
	}
	if isAllowedEmailDomain("student@bza.edu.cn", nil) {
		t.Fatal("empty allowlist must fail closed")
	}
}
