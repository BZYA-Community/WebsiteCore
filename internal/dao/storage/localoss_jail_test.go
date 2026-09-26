// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package storage

import (
	"path/filepath"
	"strings"
	"testing"
)

// testLocalOSSRoot 模拟实际部署中的存储根目录(多层相对 savePath + bucket 后缀)
const testLocalOSSRoot = "/srv/app/custom/data/paopao-ce/oss/paopao/"

func TestJailPathRejectsTraversal(t *testing.T) {
	// Issue #25 中的越狱输入, 全部必须被拒绝
	evilKeys := []string{
		"../../custom/config.yaml",        // 裸键
		"../../../../../config.yaml",      // 复现步骤中的删库键
		"../../../etc/passwd",             // 逃逸出存储根
		"a/../../../../etc/passwd",        // 先下潜再逃逸
		"../paopao-escape/tmp/escape.jpg", // 逃出 bucket 目录
		"attachment/../../../../evil.txt", // 从合法前缀目录逃逸
		"",                                // 空键: 会命中根目录本身
		".",                               // 根目录本身: os.Remove 会删掉整个存储目录
		"..",                              // 根目录的父目录
		"/../../custom/config.yaml",       // 绝对化的相对逃逸键
		"/../../../../../../etc/shadow",   // 绝对前缀 + 深层逃逸
	}
	for _, key := range evilKeys {
		if got, err := jailPath(testLocalOSSRoot, key); err == nil {
			t.Errorf("jailPath(%q) 应被拒绝, 却返回 %q", key, got)
		}
	}
}

func TestJailPathAcceptsValidKeys(t *testing.T) {
	validKeys := []string{
		"2024/06/01/0f0f8c1e-uuid.jpg",
		"attachment/2024/06/0a1b2c3d.zip",
		"avatar/user12.png",
		"sub/../other.jpg", // 含 ".." 但清理后仍在根内, 属合法键
	}
	for _, key := range validKeys {
		got, err := jailPath(testLocalOSSRoot, key)
		if err != nil {
			t.Errorf("jailPath(%q) 应被接受, 却报错: %v", key, err)
			continue
		}
		if !strings.HasPrefix(got, testLocalOSSRoot) {
			t.Errorf("jailPath(%q) = %q, 未位于根目录 %q 之内", key, got, testLocalOSSRoot)
		}
	}
}

func TestJailPathRootItselfIsCleaned(t *testing.T) {
	// root 自身带尾部斜杠/未规范化时, jail 逻辑保持一致(仍然拒绝空键)
	if _, err := jailPath("/srv/app//custom/../oss/", "."); err == nil {
		t.Error("规范化后的根目录本身必须被拒绝")
	}
}

func TestObjectKeyJailRejectsExploitVariants(t *testing.T) {
	const domain = "https://oss.example.com/oss/paopao/"
	svc := &localossServant{domain: domain}

	// 变体1: 完整URL(域名前缀通过 CheckAttachment, 残余含 "..")
	urlKey := domain + "../../custom/config.yaml"
	key := svc.ObjectKey(urlKey)
	if _, err := jailPath(testLocalOSSRoot, key); err == nil {
		t.Errorf("ObjectKey(%q) -> %q 应被 jail 拒绝", urlKey, key)
	}

	// 变体2: 不带域名的裸键
	bareKey := "../../custom/config.yaml"
	if _, err := jailPath(testLocalOSSRoot, bareKey); err == nil {
		t.Errorf("jailPath(%q) 应被拒绝", bareKey)
	}

	// 同域合法附件: ObjectKey + jail 后必须仍然可用(不回归)
	goodURL := domain + "2024/06/01/0f0f8c1e-uuid.jpg"
	goodKey := svc.ObjectKey(goodURL)
	if goodKey != "2024/06/01/0f0f8c1e-uuid.jpg" {
		t.Errorf("ObjectKey(%q) = %q, 期望剥离域名后的相对键", goodURL, goodKey)
	}
	if _, err := jailPath(testLocalOSSRoot, goodKey); err != nil {
		t.Errorf("合法键 %q 不应被 jail 拒绝: %v", goodKey, err)
	}
}

func TestJailPathMatchesReadSideBehavior(t *testing.T) {
	// 与读侧 serveLocalOSSObject 相同的语义: Clean 后必须仍在 root 之内
	key := filepath.Join("..", "..", "escape.txt")
	full, err := jailPath(testLocalOSSRoot, key)
	if err == nil {
		t.Errorf("Clean(%q) = %q, 应被拒绝", key, full)
	}
}
