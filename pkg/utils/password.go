// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package utils

import "golang.org/x/crypto/bcrypt"

// HashPassword 使用bcrypt加密密码(自带随机盐, 输出60字符)
func HashPassword(password string) string {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return ""
	}
	return string(hash)
}

// ComparePassword 校验bcrypt密码是否一致(内部为恒定时间比较)
func ComparePassword(secret, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(secret), []byte(password)) == nil
}
