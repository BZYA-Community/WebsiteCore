package security

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type smsTransport func(*http.Request) (*http.Response, error)

func (f smsTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func testAliyunSMS(transport smsTransport) *aliyunSmsServant {
	return &aliyunSmsServant{
		accessKeyID: "test-key-id", accessKeySecret: "test-key-secret",
		signName: "测试 签名+*~", templateCode: "SMS_TEST",
		client: &http.Client{Timeout: time.Second, Transport: transport},
	}
}

func TestAliyunSMSSignatureOfficialVector(t *testing.T) {
	// Fixed parameters from the official ACS3 signing example.
	req, err := http.NewRequest(http.MethodPost, "https://ecs.cn-shanghai.aliyuncs.com/?ImageId=win2019_1809_x64_dtc_zh-cn_40G_alibase_20230811.vhd&RegionId=cn-shanghai", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("x-acs-action", "RunInstances")
	req.Header.Set("x-acs-version", "2014-05-26")
	req.Header.Set("x-acs-date", "2023-10-26T10:22:32Z")
	req.Header.Set("x-acs-signature-nonce", "3156853299f313e23d1673dc12e1703d")
	signAliyunRequest(req, "YourAccessKeyId", "YourAccessKeySecret")
	want := "ACS3-HMAC-SHA256 Credential=YourAccessKeyId,SignedHeaders=host;x-acs-action;x-acs-content-sha256;x-acs-date;x-acs-signature-nonce;x-acs-version,Signature=06563a9e1b43f5dfe96b81484da74bceab24a1d853912eee15083a6f0f3283c0"
	if got := req.Header.Get("Authorization"); got != want {
		t.Fatalf("signature differs from official vector: %s", got)
	}
}

func TestAliyunSMSRequest(t *testing.T) {
	calls := 0
	service := testAliyunSMS(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.Method != http.MethodPost || r.URL.Scheme != "https" || r.URL.Host != "dysmsapi.aliyuncs.com" || r.URL.Path != "/" || r.Body != nil {
			t.Fatal("unexpected SendSms request target or body")
		}
		query := r.URL.Query()
		if query.Get("PhoneNumbers") != "13800000000" || query.Get("SignName") != "测试 签名+*~" || query.Get("TemplateCode") != "SMS_TEST" {
			t.Fatal("SMS parameters were not preserved")
		}
		if strings.Contains(r.URL.RawQuery, "+") || !strings.Contains(r.URL.RawQuery, "%20") || !strings.Contains(r.URL.RawQuery, "%2B%2A~") {
			t.Fatal("query is not RFC3986 encoded")
		}
		var params map[string]string
		if err := json.Unmarshal([]byte(query.Get("TemplateParam")), &params); err != nil || len(params) != 1 || params["code"] != "001234" {
			t.Fatalf("invalid template parameters: %v", err)
		}
		if r.Header.Get("x-acs-action") != "SendSms" || r.Header.Get("x-acs-version") != "2017-05-25" || r.Header.Get("x-acs-signature-nonce") == "" || !strings.HasPrefix(r.Header.Get("Authorization"), "ACS3-HMAC-SHA256 Credential=test-key-id,") {
			t.Fatal("missing ACS3 authentication")
		}
		date, err := time.Parse(time.RFC3339, r.Header.Get("x-acs-date"))
		if err != nil || time.Since(date).Abs() > 5*time.Second {
			t.Fatal("invalid request timestamp")
		}
		if strings.Contains(r.URL.RawQuery, "test-key") || strings.Contains(r.Header.Get("Authorization"), "test-key-secret") {
			t.Fatal("request contains exposed credentials")
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"Code":"OK","BizId":"synthetic"}`))}, nil
	})
	if err := service.SendPhoneCaptcha("13800000000", "001234", 5*time.Minute); err != nil || calls != 1 {
		t.Fatalf("SendPhoneCaptcha failed: %v, calls=%d", err, calls)
	}
}

func TestAliyunSMSFailures(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		err    error
	}{
		{"provider rejection", 200, `{"Code":"isv.BUSINESS_LIMIT_CONTROL","Message":"private phone or code"}`, nil},
		{"missing code", 200, `{}`, nil},
		{"invalid JSON", 200, `not JSON`, nil},
		{"HTTP failure", 503, `private response`, nil},
		{"network failure", 0, "", errors.New("private URL or credentials")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			service := testAliyunSMS(func(*http.Request) (*http.Response, error) {
				calls++
				if tc.err != nil {
					return nil, tc.err
				}
				return &http.Response{StatusCode: tc.status, Body: io.NopCloser(strings.NewReader(tc.body))}, nil
			})
			err := service.SendPhoneCaptcha("13800000000", "123456", time.Minute)
			if err == nil || calls != 1 || strings.Contains(err.Error(), "private") {
				t.Fatalf("failure should be sanitized and never retried: %v, calls=%d", err, calls)
			}
		})
	}
}

func TestAliyunSMSRequestTimesOut(t *testing.T) {
	calls := 0
	service := testAliyunSMS(func(r *http.Request) (*http.Response, error) {
		calls++
		<-r.Context().Done()
		return nil, r.Context().Err()
	})
	service.client.Timeout = 20 * time.Millisecond
	started := time.Now()
	if err := service.SendPhoneCaptcha("13800000000", "123456", time.Minute); err == nil || calls != 1 {
		t.Fatalf("expected one timed out request: %v, calls=%d", err, calls)
	}
	if time.Since(started) > time.Second {
		t.Fatal("SMS request timeout was not enforced")
	}
}

func TestAliyunSMSUnconfigured(t *testing.T) {
	for _, field := range []string{"id", "secret", "sign", "template"} {
		t.Run(field, func(t *testing.T) {
			service := testAliyunSMS(func(*http.Request) (*http.Response, error) {
				t.Fatal("unconfigured SMS provider made a request")
				return nil, nil
			})
			switch field {
			case "id":
				service.accessKeyID = ""
			case "secret":
				service.accessKeySecret = ""
			case "sign":
				service.signName = ""
			case "template":
				service.templateCode = ""
			}
			if err := service.SendPhoneCaptcha("13800000000", "123456", time.Minute); err == nil {
				t.Fatal("unconfigured SMS provider accepted delivery")
			}
		})
	}
	service := newAliyunSmsServant()
	if service.client.Timeout != aliyunSMSRequestTimeout || service.client.CheckRedirect == nil {
		t.Fatal("SMS client must enforce timeout and reject redirects")
	}
}
