// Copyright 2026 BZYA Community. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package security

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestAliMailSendEmailCaptchaCreatesAndSendsDraft(t *testing.T) {
	var tokenCalls, draftCalls, sendCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/oauth2/v2.0/token":
			tokenCalls.Add(1)
			if err := r.ParseForm(); err != nil || r.Form.Get("client_secret") != "secret" {
				t.Fatalf("invalid token form: %v %v", r.Form, err)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "token", "expires_in": 3600})
		case r.URL.Path == "/v2/users/sender@example.edu/messages":
			draftCalls.Add(1)
			if r.Header.Get("Authorization") != "Bearer token" {
				t.Fatalf("missing bearer token")
			}
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			encoded, _ := json.Marshal(body)
			if !strings.Contains(string(encoded), "123456") || !strings.Contains(string(encoded), "student@example.com") {
				t.Fatalf("draft lacks recipient or code: %s", encoded)
			}
			if strings.Contains(string(encoded), "少年学院") || !strings.Contains(string(encoded), "Example Community") {
				t.Fatalf("draft contains hard-coded branding or misses configured name: %s", encoded)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"message": map[string]string{"id": "draft-id"}})
		case r.URL.Path == "/v2/users/sender@example.edu/messages/draft-id/send":
			sendCalls.Add(1)
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	service := &aliMailEmailServant{
		baseURL: server.URL, clientID: "client", clientSecret: "secret",
		senderEmail: "sender@example.edu", senderName: "Example Community", client: server.Client(),
	}
	for i := 0; i < 2; i++ {
		if err := service.SendEmailCaptcha("student@example.com", "123456", 5); err != nil {
			t.Fatalf("SendEmailCaptcha() error = %v", err)
		}
	}
	if tokenCalls.Load() != 1 || draftCalls.Load() != 2 || sendCalls.Load() != 2 {
		t.Fatalf("calls token/draft/send = %d/%d/%d", tokenCalls.Load(), draftCalls.Load(), sendCalls.Load())
	}
}

func TestAliMailSendEmailCaptchaFailsClosedWithoutCredentials(t *testing.T) {
	service := &aliMailEmailServant{client: http.DefaultClient}
	if err := service.SendEmailCaptcha("student@example.com", "123456", 5); err == nil {
		t.Fatal("SendEmailCaptcha() succeeded without credentials")
	}
}
