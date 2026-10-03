// Copyright 2026 BZYA Community. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package security

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/BZYA-Community/WebsiteCore/internal/conf"
)

type aliMailToken struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"`
}

type aliMailDraft struct {
	Message struct {
		ID string `json:"id"`
	} `json:"message"`
}

type aliMailError struct {
	DetailErrorCode string `json:"detailErrorCode"`
	Message         string `json:"message"`
}

type aliMailEmailServant struct {
	baseURL, clientID, clientSecret, senderEmail, senderName string
	client                                                   *http.Client
	mu                                                       sync.Mutex
	token                                                    string
	tokenExpiresAt                                           time.Time
}

func NewEmailVerifyService() *aliMailEmailServant {
	cfg := conf.AliMailSetting
	if cfg == nil {
		return &aliMailEmailServant{client: &http.Client{Timeout: 15 * time.Second}}
	}
	return &aliMailEmailServant{
		baseURL:      strings.TrimRight(cfg.BaseURL, "/"),
		clientID:     cfg.ClientID,
		clientSecret: cfg.ClientSecret,
		senderEmail:  cfg.SenderEmail,
		senderName:   cfg.SenderName,
		client:       &http.Client{Timeout: 15 * time.Second},
	}
}

func (s *aliMailEmailServant) SendEmailCaptcha(email, captcha string, expireMinutes int) error {
	if s.baseURL == "" || s.clientID == "" || s.clientSecret == "" || s.senderEmail == "" {
		return errors.New("AliMail email verification is not configured")
	}
	token, err := s.accessToken(context.Background())
	if err != nil {
		return err
	}
	siteName := strings.TrimSpace(s.senderName)
	if siteName == "" {
		siteName = "WebsiteCore"
	}
	bodyText := fmt.Sprintf("你的 %s 验证码是 %s，有效期 %d 分钟。请勿将验证码告诉他人。", siteName, captcha, expireMinutes)
	payload := map[string]any{"message": map[string]any{
		"subject":      siteName + " 邮箱验证码",
		"from":         map[string]string{"email": s.senderEmail, "name": siteName},
		"toRecipients": []map[string]string{{"email": email}},
		"body": map[string]string{
			"bodyText": bodyText,
			"bodyHtml": fmt.Sprintf("<p>你的 %s 验证码是：</p><p style=\"font-size:28px;font-weight:700;letter-spacing:4px\">%s</p><p>有效期 %d 分钟。请勿将验证码告诉他人。</p>", html.EscapeString(siteName), html.EscapeString(captcha), expireMinutes),
		},
	}}
	var draft aliMailDraft
	path := "/v2/users/" + url.PathEscape(s.senderEmail) + "/messages"
	if err := s.requestJSON(context.Background(), token, http.MethodPost, path, payload, &draft); err != nil {
		return err
	}
	if draft.Message.ID == "" {
		return errors.New("AliMail create draft returned no message id")
	}
	sendPath := path + "/" + url.PathEscape(draft.Message.ID) + "/send"
	return s.requestJSON(context.Background(), token, http.MethodPost, sendPath, map[string]bool{"saveToSentItems": true}, nil)
}

func (s *aliMailEmailServant) accessToken(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.token != "" && time.Now().Add(time.Minute).Before(s.tokenExpiresAt) {
		return s.token, nil
	}
	form := url.Values{"grant_type": {"client_credentials"}, "client_id": {s.clientID}, "client_secret": {s.clientSecret}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/oauth2/v2.0/token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("AliMail token request failed: HTTP %d", resp.StatusCode)
	}
	var result aliMailToken
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if result.AccessToken == "" || result.ExpiresIn <= 0 {
		return "", errors.New("AliMail token response is incomplete")
	}
	s.token, s.tokenExpiresAt = result.AccessToken, time.Now().Add(time.Duration(result.ExpiresIn)*time.Second)
	return s.token, nil
}

func (s *aliMailEmailServant) requestJSON(ctx context.Context, token, method, path string, payload, dst any) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, method, s.baseURL+path, strings.NewReader(string(encoded)))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiErr aliMailError
		_ = json.NewDecoder(resp.Body).Decode(&apiErr)
		return fmt.Errorf("AliMail request failed: HTTP %d code=%s message=%s", resp.StatusCode, apiErr.DetailErrorCode, apiErr.Message)
	}
	if dst != nil {
		return json.NewDecoder(resp.Body).Decode(dst)
	}
	return nil
}
