// Copyright 2026 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package avatar 默认头像生成: 基于 identicon(GitHub 同款 5x5 像素块风格)
// 按种子(用户名等)确定性生成 PNG 并存入本站对象存储, 杜绝外链默认头像。
package avatar

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/BZYA-Community/WebsiteCore/internal/dao"
	"github.com/rrivera/identicon"
)

const (
	// namespace 参与哈希, 固定命名空间保证同一种子生成结果稳定
	namespace = "paopao-ce"
	// gridSize 5x5 网格, 与 GitHub identicon 一致
	gridSize = 5
	// density 像素块密度(1-5), 3 为库推荐的均衡值
	density = 3
	// pngSize 输出 PNG 边长(像素)
	pngSize = 300
	// objectPrefix 默认头像在对象存储中的目录前缀
	objectPrefix = "public/avatar/default/"
)

// Generate 按种子生成 GitHub 风格 identicon 头像, 存入对象存储并返回可访问 URL。
// 同一种子幂等: 对象已存在时直接返回其 URL, 不重复写入。
func Generate(seed string) (string, error) {
	ig, err := identicon.New(namespace, gridSize, density)
	if err != nil {
		return "", fmt.Errorf("avatar.Generate new generator: %w", err)
	}
	ii, err := ig.Draw(seed)
	if err != nil {
		return "", fmt.Errorf("avatar.Generate draw %q: %w", seed, err)
	}
	buf := &bytes.Buffer{}
	if err = ii.Png(pngSize, buf); err != nil {
		return "", fmt.Errorf("avatar.Generate encode png %q: %w", seed, err)
	}

	sum := sha256.Sum256([]byte(namespace + ":" + seed))
	objectKey := objectPrefix + hex.EncodeToString(sum[:16]) + ".png"

	oss := dao.ObjectStorageService()
	if exist, eerr := oss.IsObjectExist(objectKey); eerr == nil && exist {
		return oss.ObjectURL(objectKey), nil
	}
	data := buf.Bytes()
	if _, err = oss.PutObject(objectKey, bytes.NewReader(data), int64(len(data)), "image/png", true); err != nil {
		return "", fmt.Errorf("avatar.Generate put object %q: %w", objectKey, err)
	}
	return oss.ObjectURL(objectKey), nil
}
