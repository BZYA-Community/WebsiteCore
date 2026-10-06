package web

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/BZYA-Community/WebsiteCore/internal/conf"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/storage"
)

func TestCourseUploadCapabilities(t *testing.T) {
	oldJWT, oldOSS := conf.JWTSetting, conf.AliOSSSetting
	t.Cleanup(func() { conf.JWTSetting, conf.AliOSSSetting = oldJWT, oldOSS })
	conf.JWTSetting, conf.AliOSSSetting = nil, nil
	if err := json.Unmarshal([]byte(`{"Secret":"test-only-upload-secret"}`), &conf.JWTSetting); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(`{"Endpoint":"oss-cn-hangzhou.aliyuncs.com","Bucket":"test-bucket","AccessKeyID":"test-key","AccessKeySecret":"test-only-secret"}`), &conf.AliOSSSetting); err != nil {
		t.Fatal(err)
	}
	a := &ms.Attachment{Model: &ms.Model{ID: 42}, Content: "attachment/course/staging/test.pdf", MimeType: "application/pdf", FileSize: 2 << 30, UploadExpiresOn: time.Now().Unix() + 60}
	local, err := courseUploadCredential(a, false)
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(local.UploadURL)
	if err != nil {
		t.Fatal(err)
	}
	expires, err := strconv.ParseInt(u.Query().Get("expired"), 10, 64)
	if err != nil || local.Method != "PUT" || local.Mode != "direct" || !storage.VerifyLocalOSSSign("PUT:"+u.Path, expires, u.Query().Get("sign")) {
		t.Fatal("invalid local capability")
	}
	if storage.VerifyLocalOSSSign(u.Path, expires, u.Query().Get("sign")) || storage.VerifyLocalOSSSign("PUT:"+u.Path+"0", expires, u.Query().Get("sign")) || storage.VerifyLocalOSSSign("PUT:"+u.Path, expires+1, u.Query().Get("sign")) {
		t.Fatal("capability not bound to method/path/expiry")
	}
	ali, err := courseUploadCredential(a, true)
	if err != nil {
		t.Fatal(err)
	}
	if ali.Method != "POST" || ali.UploadURL != "https://test-bucket.oss-cn-hangzhou.aliyuncs.com" {
		t.Fatal("unexpected AliOSS destination")
	}
	mac := hmac.New(sha1.New, []byte(conf.AliOSSSetting.AccessKeySecret))
	mac.Write([]byte(ali.Fields["policy"]))
	if base64.StdEncoding.EncodeToString(mac.Sum(nil)) != ali.Fields["Signature"] {
		t.Fatal("policy signature mismatch")
	}
	data, err := base64.StdEncoding.DecodeString(ali.Fields["policy"])
	if err != nil {
		t.Fatal(err)
	}
	var policy struct {
		Expiration string
		Conditions []json.RawMessage
	}
	if err := json.Unmarshal(data, &policy); err != nil {
		t.Fatal(err)
	}
	if policy.Expiration != time.Unix(a.UploadExpiresOn, 0).UTC().Format(time.RFC3339) {
		t.Fatal("wrong policy expiry")
	}
	exact := map[string]string{}
	sizeBound, bucketBound := false, false
	for _, raw := range policy.Conditions {
		var condition []any
		if json.Unmarshal(raw, &condition) == nil && len(condition) == 3 {
			if condition[0] == "eq" {
				exact[condition[1].(string)] = condition[2].(string)
			}
			if condition[0] == "content-length-range" {
				sizeBound = condition[1] == float64(a.FileSize) && condition[2] == float64(a.FileSize)
			}
		} else {
			var bound map[string]string
			if json.Unmarshal(raw, &bound) == nil && bound["bucket"] == "test-bucket" {
				bucketBound = true
			}
		}
	}
	if !sizeBound || !bucketBound || exact["$key"] != a.Content || exact["$Content-Type"] != a.MimeType || exact["$x-oss-object-acl"] != "private" || exact["$x-oss-forbid-overwrite"] != "true" || exact["$success_action_status"] != "200" {
		t.Fatal("policy does not bind exact private upload")
	}
}
