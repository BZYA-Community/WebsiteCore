package app

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/BZYA-Community/WebsiteCore/internal/conf"
	"github.com/golang-jwt/jwt/v5"
)

func TestParseTokenValidation(t *testing.T) {
	previous := conf.JWTSetting
	conf.JWTSetting = nil
	if err := json.Unmarshal([]byte(`{"Secret":"jwt-validation-test-secret"}`), &conf.JWTSetting); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conf.JWTSetting = previous })

	for _, tc := range []struct {
		name   string
		method jwt.SigningMethod
		expiry *jwt.NumericDate
		secret string
		want   error
	}{
		{"valid", jwt.SigningMethodHS256, jwt.NewNumericDate(time.Now().Add(time.Hour)), conf.JWTSetting.Secret, nil},
		{"expired", jwt.SigningMethodHS256, jwt.NewNumericDate(time.Now().Add(-time.Hour)), conf.JWTSetting.Secret, jwt.ErrTokenExpired},
		{"missing expiry", jwt.SigningMethodHS256, nil, conf.JWTSetting.Secret, jwt.ErrTokenRequiredClaimMissing},
		{"wrong algorithm", jwt.SigningMethodHS512, jwt.NewNumericDate(time.Now().Add(time.Hour)), conf.JWTSetting.Secret, jwt.ErrTokenSignatureInvalid},
		{"wrong signature", jwt.SigningMethodHS256, jwt.NewNumericDate(time.Now().Add(time.Hour)), "different-secret", jwt.ErrTokenSignatureInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			claims := Claims{UID: 7, RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: tc.expiry}}
			token, err := jwt.NewWithClaims(tc.method, claims).SignedString([]byte(tc.secret))
			if err != nil {
				t.Fatal(err)
			}
			got, err := ParseToken(token)
			if !errors.Is(err, tc.want) {
				t.Fatalf("ParseToken() error = %v, want %v", err, tc.want)
			}
			if tc.want == nil && (got == nil || got.UID != claims.UID) {
				t.Fatalf("ParseToken() claims = %v", got)
			}
			if tc.want != nil && got != nil {
				t.Fatal("invalid token returned claims")
			}
		})
	}
}
