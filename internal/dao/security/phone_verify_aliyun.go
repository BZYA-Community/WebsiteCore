package security

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/BZYA-Community/WebsiteCore/internal/conf"
)

const aliyunSMSRequestTimeout = 10 * time.Second

type aliyunSmsServant struct {
	accessKeyID     string
	accessKeySecret string
	signName        string
	templateCode    string
	client          *http.Client
}

func newAliyunSmsServant() *aliyunSmsServant {
	s := &aliyunSmsServant{client: &http.Client{
		Timeout:       aliyunSMSRequestTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}
	if c := conf.SmsAliyunSetting; c != nil {
		s.accessKeyID = strings.TrimSpace(c.AccessKeyID)
		s.accessKeySecret = strings.TrimSpace(c.AccessKeySecret)
		s.signName = strings.TrimSpace(c.SignName)
		s.templateCode = strings.TrimSpace(c.TemplateCode)
	}
	return s
}

// SendPhoneCaptcha uses the approved ${code} template; expiry is enforced locally.
func (s *aliyunSmsServant) SendPhoneCaptcha(phone, captcha string, _ time.Duration) error {
	if s.accessKeyID == "" || s.accessKeySecret == "" || s.signName == "" || s.templateCode == "" {
		return fmt.Errorf("aliyun SMS provider is not configured")
	}
	params, err := json.Marshal(map[string]string{"code": captcha})
	if err != nil {
		return err
	}
	query := url.Values{
		"PhoneNumbers": {phone}, "SignName": {s.signName},
		"TemplateCode": {s.templateCode}, "TemplateParam": {string(params)},
	}.Encode()
	req, err := http.NewRequest(http.MethodPost, "https://dysmsapi.aliyuncs.com/?"+strings.ReplaceAll(query, "+", "%20"), nil)
	if err != nil {
		return fmt.Errorf("aliyun SMS request could not be created")
	}
	req.Header.Set("x-acs-action", "SendSms")
	req.Header.Set("x-acs-version", "2017-05-25")
	req.Header.Set("x-acs-date", time.Now().UTC().Format(time.RFC3339))
	req.Header.Set("x-acs-signature-nonce", rand.Text())
	signAliyunRequest(req, s.accessKeyID, s.accessKeySecret)
	// SendSms has no idempotency guarantee: an ambiguous failure must not be retried.
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("aliyun SMS provider request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("aliyun SMS provider returned HTTP %d", resp.StatusCode)
	}
	var result struct{ Code string }
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&result); err != nil || result.Code == "" {
		return fmt.Errorf("invalid aliyun SMS provider response")
	}
	if result.Code != "OK" {
		return fmt.Errorf("aliyun SMS provider rejected request (%s)", result.Code)
	}
	return nil
}

// ACS3 signing: https://help.aliyun.com/zh/sdk/product-overview/v3-request-structure-and-signature
func signAliyunRequest(req *http.Request, accessKeyID, accessKeySecret string) {
	const signedHeaders = "host;x-acs-action;x-acs-content-sha256;x-acs-date;x-acs-signature-nonce;x-acs-version"
	payloadHash := fmt.Sprintf("%x", sha256.Sum256(nil))
	req.Header.Set("x-acs-content-sha256", payloadHash)
	canonicalHeaders := "host:" + req.URL.Host + "\n"
	for _, name := range strings.Split(signedHeaders, ";")[1:] {
		canonicalHeaders += name + ":" + strings.TrimSpace(req.Header.Get(name)) + "\n"
	}
	canonicalRequest := req.Method + "\n/\n" + req.URL.RawQuery + "\n" + canonicalHeaders + "\n" + signedHeaders + "\n" + payloadHash
	stringToSign := fmt.Sprintf("ACS3-HMAC-SHA256\n%x", sha256.Sum256([]byte(canonicalRequest)))
	mac := hmac.New(sha256.New, []byte(accessKeySecret))
	mac.Write([]byte(stringToSign))
	req.Header.Set("Authorization", fmt.Sprintf("ACS3-HMAC-SHA256 Credential=%s,SignedHeaders=%s,Signature=%x", accessKeyID, signedHeaders, mac.Sum(nil)))
}
