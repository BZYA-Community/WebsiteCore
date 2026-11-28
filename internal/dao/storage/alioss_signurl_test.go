package storage

import (
	"strings"
	"testing"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

// SignURL必须总是给浏览器https链接: Endpoint不写scheme时SDK默认拼http://，
// https站点会把签名下载链接当混合内容拦截(帖子/课程/专栏附件)。
func TestAliossSignURLForcesHTTPS(t *testing.T) {
	for _, endpoint := range []string{"oss-cn-hangzhou.aliyuncs.com", "http://oss-cn-hangzhou.aliyuncs.com", "https://oss-cn-hangzhou.aliyuncs.com"} {
		client, err := oss.New(endpoint, "test-access", "test-secret")
		if err != nil {
			t.Fatalf("endpoint %q: %v", endpoint, err)
		}
		bucket, err := client.Bucket("test-bucket")
		if err != nil {
			t.Fatalf("endpoint %q: %v", endpoint, err)
		}
		signed, err := (&aliossServant{bucket: bucket}).SignURL("attachment/course/1.mp4", 3600)
		if err != nil {
			t.Fatalf("endpoint %q: %v", endpoint, err)
		}
		// 协议强制https，主机/路径(对象键)/查询参数原样保留(签名只覆盖方法/路径/参数，换协议不影响)
		if !strings.HasPrefix(signed, "https://test-bucket.oss-cn-hangzhou.aliyuncs.com/") || !strings.Contains(signed, "1.mp4") {
			t.Errorf("endpoint %q signed URL not https or object key mangled: %s", endpoint, signed)
		}
		if !strings.Contains(strings.ToLower(signed), "signature=") {
			t.Errorf("endpoint %q signed URL lost signature params: %s", endpoint, signed)
		}
	}
}
