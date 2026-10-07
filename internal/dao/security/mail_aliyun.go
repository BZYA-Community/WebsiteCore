package security

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/BZYA-Community/WebsiteCore/internal/conf"
)

// AliyunMail uses the enterprise-mail OAuth API requested by the deployment.
// Documentation: https://help.aliyun.com/zh/document_detail/2856076.html
type AliyunMail struct {
	config  conf.AliyunMailConf
	client  *http.Client
	mu      sync.Mutex
	token   string
	expires time.Time
}

func NewAliyunMail(config conf.AliyunMailConf) *AliyunMail {
	return &AliyunMail{config: config, client: &http.Client{Timeout: 10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

func (s *AliyunMail) request(ctx context.Context, path, contentType, token string, body io.Reader, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(s.config.Endpoint, "/")+path, body)
	if err != nil {
		return fmt.Errorf("mail request could not be created")
	}
	req.Header.Set("Content-Type", contentType)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("mail provider request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("mail provider returned HTTP %d", resp.StatusCode)
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(out); err != nil {
		return fmt.Errorf("invalid mail provider response")
	}
	return nil
}

func (s *AliyunMail) accessToken(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.token != "" && time.Now().Before(s.expires) {
		return s.token, nil
	}
	form := url.Values{"grant_type": {"client_credentials"}, "client_id": {s.config.ClientID}, "client_secret": {s.config.ClientSecret}}
	var token struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := s.request(ctx, "/oauth2/v2.0/token", "application/x-www-form-urlencoded", "", strings.NewReader(form.Encode()), &token); err != nil {
		return "", err
	}
	if token.AccessToken == "" || token.ExpiresIn <= 0 || token.ExpiresIn > 30*24*3600 {
		return "", fmt.Errorf("invalid mail access token response")
	}
	s.token = token.AccessToken
	s.expires = time.Now().Add(time.Duration(max(token.ExpiresIn-60, 1)) * time.Second)
	return s.token, nil
}

func (s *AliyunMail) SendCode(ctx context.Context, address, code string, ttl time.Duration) error {
	if s.config.ClientID == "" || s.config.ClientSecret == "" || s.config.Sender == "" {
		return fmt.Errorf("email verification provider is not configured")
	}
	token, err := s.accessToken(ctx)
	if err != nil {
		return err
	}
	body, err := json.Marshal(map[string]any{"message": map[string]any{
		"subject":      "WebsiteCore verification code / 验证码",
		"from":         map[string]string{"email": s.config.Sender},
		"toRecipients": []map[string]string{{"email": address}},
		"body":         map[string]string{"bodyText": fmt.Sprintf("Your verification code / 您的验证码: %s\nValid for %d minutes / %d 分钟内有效。\nIf you did not request this code, ignore this email. / 如非本人操作，请忽略。", code, int(ttl.Minutes()), int(ttl.Minutes()))},
	}})
	if err != nil {
		return err
	}
	path := "/v2/users/" + url.PathEscape(s.config.Sender) + "/messages"
	var draft struct {
		Message struct {
			ID string `json:"id"`
		} `json:"message"`
	}
	if err := s.request(ctx, path, "application/json", token, bytes.NewReader(body), &draft); err != nil {
		return err
	}
	if draft.Message.ID == "" {
		return fmt.Errorf("mail provider returned no draft ID")
	}
	return s.request(ctx, path+"/"+url.PathEscape(draft.Message.ID)+"/send", "application/json", token, strings.NewReader(`{"saveToSentItems":true}`), nil)
}
