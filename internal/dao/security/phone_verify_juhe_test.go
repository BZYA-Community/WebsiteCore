package security

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gopkg.in/resty.v1"
)

func TestJuheSMSRequestTimesOut(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(150 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"error_code":0,"reason":"ok"}`))
	}))
	defer server.Close()

	client := resty.New().SetTimeout(25 * time.Millisecond)
	service := &juheSmsServant{gateway: server.URL, client: client, tplVal: "%s %s"}
	started := time.Now()
	if err := service.SendPhoneCaptcha("13800000000", "123456", time.Minute); err == nil {
		t.Fatal("SendPhoneCaptcha() error = nil, want timeout")
	}
	if elapsed := time.Since(started); elapsed >= 140*time.Millisecond {
		t.Fatalf("request elapsed %s, timeout was not enforced", elapsed)
	}
}

func TestJuheSMSParsesSuccessfulResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"error_code":0,"reason":"ok"}`))
	}))
	defer server.Close()

	service := &juheSmsServant{gateway: server.URL, client: resty.New().SetTimeout(time.Second), tplVal: "%s %s"}
	if err := service.SendPhoneCaptcha("13800000000", "123456", time.Minute); err != nil {
		t.Fatalf("SendPhoneCaptcha() error = %v", err)
	}
}
