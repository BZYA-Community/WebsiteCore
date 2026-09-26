// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package storage

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BZYA-Community/WebsiteCore/internal/conf"
	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/Masterminds/semver/v3"
	"github.com/cockroachdb/errors"
	"github.com/sirupsen/logrus"
)

type localossCreateServant struct {
	savePath string
	domain   string
}

type localossCreateTempDirServant struct {
	savePath string
	domain   string
	tempDir  string
}

type localossServant struct {
	core.OssCreateService

	savePath string
	domain   string
}

// jailPath 将对象键安全地拼接到存储根目录, 防止路径越狱(#25)。
// root 与 key 均先规范化(root 做 Abs/Clean, key 做 Clean 后 Join),
// 拼接结果必须严格位于 root 之内, 否则返回错误由调用方拒绝该操作。
// 拒绝的典型输入: 含 ".." 逃逸的键、空键、以及恰好等于根目录本身的键
// (后者若放行会被 os.Remove 删掉整个存储目录)。
func jailPath(root, key string) (string, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve localoss root %q: %w", root, err)
	}
	fullPath := filepath.Clean(filepath.Join(absRoot, key))
	if fullPath == absRoot || !strings.HasPrefix(fullPath, absRoot+string(os.PathSeparator)) {
		return "", fmt.Errorf("object key escapes storage root: %q", key)
	}
	return fullPath, nil
}

func (s *localossCreateServant) PutObject(objectKey string, reader io.Reader, objectSize int64, contentType string, _persistance bool) (string, error) {
	savePath, err := jailPath(s.savePath, objectKey)
	if err != nil {
		return "", err
	}
	if err = os.MkdirAll(filepath.Dir(savePath), 0o750); err != nil && !os.IsExist(err) {
		return "", err
	}

	writer, err := os.Create(savePath)
	if err != nil {
		return "", err
	}
	defer writer.Close()

	written, err := io.Copy(writer, reader)
	if err != nil {
		return "", err
	}
	if written != objectSize {
		os.Remove(savePath)
		return "", errors.New("put object not complete")
	}

	return s.domain + objectKey, nil
}

func (s *localossCreateServant) PersistObject(objectKey string) error {
	// Direct 模式无临时文件可搬, 但对象键仍需过 jail(#25), 拒绝越狱键
	_, err := jailPath(s.savePath, objectKey)
	return err
}

func (s *localossCreateTempDirServant) PutObject(objectKey string, reader io.Reader, objectSize int64, contentType string, persistance bool) (string, error) {
	// 先对原始对象键过 jail(#25), 防止 ".." 与临时目录前缀相互抵消后再落盘
	if _, err := jailPath(s.savePath, objectKey); err != nil {
		return "", err
	}
	objectName := objectKey
	if !persistance {
		objectName = s.tempDir + objectKey
	}
	savePath, err := jailPath(s.savePath, objectName)
	if err != nil {
		return "", err
	}
	if err = os.MkdirAll(filepath.Dir(savePath), 0o750); err != nil && !os.IsExist(err) {
		return "", err
	}

	writer, err := os.Create(savePath)
	if err != nil {
		return "", err
	}
	defer writer.Close()

	written, err := io.Copy(writer, reader)
	if err != nil {
		return "", err
	}
	if written != objectSize {
		os.Remove(savePath)
		return "", errors.New("put object not complete")
	}

	return s.domain + objectKey, nil
}

func (s *localossCreateTempDirServant) PersistObject(objectKey string) error {
	// 该模式会读临时文件、写正式文件并删除临时文件, 每个路径都要过 jail(#25)
	savePath, err := jailPath(s.savePath, objectKey)
	if err != nil {
		return err
	}
	tmpObjPath, err := jailPath(s.savePath, s.tempDir+objectKey)
	if err != nil {
		return err
	}

	fi, err := os.Stat(savePath)
	if err == nil && !fi.IsDir() {
		logrus.Debugf("object exist so do nothing objectKey: %s", objectKey)
		return nil
	}

	if err = os.MkdirAll(filepath.Dir(savePath), 0o750); err != nil && !os.IsExist(err) {
		return err
	}

	reader, err := os.Open(tmpObjPath)
	if err != nil {
		return err
	}
	needCloseReader := true
	defer func() {
		if needCloseReader {
			reader.Close()
		}
	}()

	writer, err := os.Create(savePath)
	if err != nil {
		return err
	}
	defer writer.Close()
	if _, err = io.Copy(writer, reader); err != nil {
		return err
	}

	reader.Close()
	needCloseReader = false
	if err = os.Remove(tmpObjPath); err != nil {
		return err
	}

	return nil
}

func (s *localossServant) DeleteObject(objectKey string) error {
	fullPath, err := jailPath(s.savePath, objectKey)
	if err != nil {
		// 拒绝越狱键并留下告警(#25), 调用方(如deleteOssObjects)容忍删除错误
		logrus.Warnf("localoss delete object refused: %v", err)
		return err
	}
	return os.Remove(fullPath)
}

func (s *localossServant) DeleteObjects(objectKeys []string) (err error) {
	// 宽松处理删除动作，尽可能删除所有objectKey，如果出错，只返回最后一个错误
	for _, objectKey := range objectKeys {
		fullPath, e := jailPath(s.savePath, objectKey)
		if e != nil {
			logrus.Warnf("localoss delete object refused: %v", e)
			err = e
			continue
		}
		if e = os.Remove(fullPath); e != nil {
			err = e
		}
	}
	return
}

func (s *localossServant) IsObjectExist(objectKey string) (bool, error) {
	fullPath, err := jailPath(s.savePath, objectKey)
	if err != nil {
		return false, err
	}
	fi, err := os.Stat(fullPath)
	if err != nil {
		return false, err
	}
	return !fi.IsDir(), nil
}

func (s *localossServant) SignURL(objectKey string, expiredInSec int64) (string, error) {
	if expiredInSec < 0 {
		return "", fmt.Errorf("invalid expires: %d, expires must bigger than 0", expiredInSec)
	}
	// 越狱键不签发链接(#25)
	if _, err := jailPath(s.savePath, objectKey); err != nil {
		return "", err
	}
	expiration := time.Now().Unix() + expiredInSec

	// 安全修复: 旧实现仅附加expired参数从不校验，等于永久公开链接；
	// 现在生成真实HMAC签名，由LocalOSS文件服务(serveLocalOSSObject)强制校验
	signedPath := objectKey
	if u, err := url.Parse(s.domain); err == nil {
		signedPath = u.Path + objectKey
	}
	uri := fmt.Sprintf("%s%s?expired=%d&sign=%s", s.domain, objectKey, expiration, LocalOSSSign(signedPath, expiration))
	return uri, nil
}

// LocalOSSSign 使用服务端密钥对"请求路径+过期时间"生成HMAC-SHA256签名，
// 密钥复用JWT密钥(服务端私有不外泄)
func LocalOSSSign(reqPath string, expired int64) string {
	secret := ""
	if conf.JWTSetting != nil {
		secret = conf.JWTSetting.Secret
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(fmt.Sprintf("%s:%d", reqPath, expired)))
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyLocalOSSSign 校验签名与有效期，恒定时间比较避免时序攻击
func VerifyLocalOSSSign(reqPath string, expired int64, sign string) bool {
	if expired < time.Now().Unix() {
		return false
	}
	expected := LocalOSSSign(reqPath, expired)
	return hmac.Equal([]byte(expected), []byte(sign))
}

func (s *localossServant) ObjectURL(objetKey string) string {
	return s.domain + objetKey
}

// ObjectKey 从完整URL中剥离存储域名得到对象键。
// 注意: 本函数保持无错误签名以兼容既有调用方, 越狱防护由 jailPath 在所有
// 落盘入口(Put/Persist/Delete/IsExist/SignURL)强制执行; 这里对剥离后仍含
// ".." 段的可疑键仅做告警, 便于排查恶意请求(#25)。
func (s *localossServant) ObjectKey(objectUrl string) string {
	key := strings.Replace(objectUrl, s.domain, "", -1)
	for _, seg := range strings.Split(key, "/") {
		if seg == ".." {
			logrus.Warnf("localoss suspicious object key contains .. : %q", key)
			break
		}
	}
	return key
}

func (s *localossServant) Name() string {
	return "LocalOSS"
}

func (s *localossServant) Version() *semver.Version {
	return semver.MustParse("v0.2.0")
}
