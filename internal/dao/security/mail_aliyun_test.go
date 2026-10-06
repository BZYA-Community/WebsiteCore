package security

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/BZYA-Community/WebsiteCore/internal/conf"
)

type mailTransport func(*http.Request) (*http.Response, error)

func (f mailTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestAliyunMailContractAndTokenReuse(t *testing.T) {
	s := NewAliyunMail(conf.AliyunMailConf{Endpoint: "https://mail.example.test", ClientID: "test-client", ClientSecret: "test-secret", Sender: "sender@example.test"})
	tokens, drafts, sends := 0, 0, 0
	s.client.Transport = mailTransport(func(r *http.Request) (*http.Response, error) {
		if r.Method != "POST" {
			t.Fatalf("unexpected method %s", r.Method)
		}
		response := `{}`
		switch r.URL.Path {
		case "/oauth2/v2.0/token":
			tokens++
			if err := r.ParseForm(); err != nil || r.Form.Get("grant_type") != "client_credentials" || r.Form.Get("client_id") != "test-client" || r.Form.Get("client_secret") != "test-secret" {
				t.Fatal("incorrect OAuth form")
			}
			response = `{"access_token":"test-token","expires_in":3600}`
		case "/v2/users/sender@example.test/messages":
			drafts++
			if r.Header.Get("Authorization") != "Bearer test-token" {
				t.Fatal("missing authorization")
			}
			var payload struct {
				Message struct {
					To   []struct{ Email string } `json:"toRecipients"`
					Body struct {
						Text string `json:"bodyText"`
					}
				}
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			if len(payload.Message.To) != 1 || payload.Message.To[0].Email != "recipient@example.test" || !strings.Contains(payload.Message.Body.Text, "123456") || !strings.Contains(payload.Message.Body.Text, "5 minutes") {
				t.Fatal("incorrect verification message")
			}
			response = `{"message":{"id":"draft-id"}}`
		case "/v2/users/sender@example.test/messages/draft-id/send":
			sends++
		default:
			t.Fatalf("unexpected mail endpoint %s", r.URL.Path)
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(response))}, nil
	})
	for range 2 {
		if err := s.SendCode(context.Background(), "recipient@example.test", "123456", 5*time.Minute); err != nil {
			t.Fatal(err)
		}
	}
	if tokens != 1 || drafts != 2 || sends != 2 {
		t.Fatalf("calls = %d/%d/%d", tokens, drafts, sends)
	}
	s.client.Transport = mailTransport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 403, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("secret provider response"))}, nil
	})
	if err := s.SendCode(context.Background(), "recipient@example.test", "123456", 5*time.Minute); err == nil || strings.Contains(err.Error(), "secret provider") {
		t.Fatal("provider failure was ignored or leaked")
	}
}
