// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package storage

import (
	"io"
	"strings"
	"time"

	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/Masterminds/semver/v3"
	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/sirupsen/logrus"
)

type aliossCreateServant struct {
	bucket *oss.Bucket
	domain string
}

type aliossCreateRetentionServant struct {
	bucket          *oss.Bucket
	domain          string
	retainInDays    time.Duration
	retainUntilDate time.Time
}

type aliossCreateTempDirServant struct {
	bucket  *oss.Bucket
	domain  string
	tempDir string
}

type aliossServant struct {
	core.OssCreateService

	bucket *oss.Bucket
	domain string
}

func (s *aliossCreateServant) PutObject(objectKey string, reader io.Reader, objectSize int64, contentType string, _persistance bool) (string, error) {
	options := []oss.Option{
		oss.ContentLength(objectSize),
		oss.ContentType(contentType),
		oss.ForbidOverWrite(true),
	}
	if strings.HasPrefix(objectKey, "attachment/") {
		options = append(options, oss.ObjectACL(oss.ACLPrivate))
	}
	err := s.bucket.PutObject(objectKey, reader, options...)
	if err != nil {
		return "", err
	}
	return s.domain + objectKey, nil
}

func (s *aliossCreateServant) PersistObject(_objectKey string) error {
	// empty
	return nil
}

func (s *aliossCreateRetentionServant) PutObject(objectKey string, reader io.Reader, objectSize int64, contentType string, persistance bool) (string, error) {
	options := []oss.Option{
		oss.ContentLength(objectSize),
		oss.ContentType(contentType),
		oss.ForbidOverWrite(true),
	}
	if strings.HasPrefix(objectKey, "attachment/") {
		options = append(options, oss.ObjectACL(oss.ACLPrivate))
	}
	if !persistance {
		options = append(options, oss.Expires(time.Now().Add(s.retainInDays)))
	}
	err := s.bucket.PutObject(objectKey, reader, options...)
	if err != nil {
		return "", err
	}
	return s.domain + objectKey, nil
}

func (s *aliossCreateRetentionServant) PersistObject(objectKey string) error {
	return s.bucket.SetObjectMeta(objectKey, oss.Expires(s.retainUntilDate))
}

func (s *aliossCreateTempDirServant) PutObject(objectKey string, reader io.Reader, objectSize int64, contentType string, persistance bool) (string, error) {
	objectName := objectKey
	if !persistance {
		objectName = s.tempDir + objectKey
	}
	options := []oss.Option{
		oss.ContentLength(objectSize),
		oss.ContentType(contentType),
		oss.ForbidOverWrite(true),
	}
	if strings.HasPrefix(objectKey, "attachment/") {
		options = append(options, oss.ObjectACL(oss.ACLPrivate))
	}
	err := s.bucket.PutObject(objectName, reader, options...)
	if err != nil {
		return "", err
	}
	return s.domain + objectKey, nil
}

func (s *aliossCreateTempDirServant) PersistObject(objectKey string) error {
	exsit, err := s.bucket.IsObjectExist(objectKey)
	if err != nil {
		return err
	}
	if exsit {
		logrus.Debugf("object exist so do nothing objectKey: %s", objectKey)
		return nil
	}
	options := []oss.Option{oss.ForbidOverWrite(true)}
	if strings.HasPrefix(objectKey, "attachment/") {
		options = append(options, oss.ObjectACL(oss.ACLPrivate))
	}
	if _, err := s.bucket.CopyObject(s.tempDir+objectKey, objectKey, options...); err != nil {
		return err
	}
	return s.bucket.DeleteObject(s.tempDir + objectKey)
}

func (s *aliossServant) DeleteObject(objectKey string) error {
	return s.bucket.DeleteObject(objectKey)
}

func (s *aliossServant) DeleteObjects(objectKeys []string) error {
	_, err := s.bucket.DeleteObjects(objectKeys)
	return err
}

func (s *aliossServant) IsObjectExist(objectKey string) (bool, error) {
	return s.bucket.IsObjectExist(objectKey)
}

func (s *aliossServant) SignURL(objectKey string, expiredInSec int64) (string, error) {
	return s.bucket.SignURL(objectKey, oss.HTTPGet, expiredInSec)
}

func (s *aliossServant) ObjectURL(objetKey string) string {
	return s.domain + objetKey
}

func (s *aliossServant) ObjectKey(objectUrl string) string {
	return strings.TrimPrefix(objectUrl, s.domain)
}

func (s *aliossServant) Name() string {
	return "AliOSS"
}

func (s *aliossServant) Version() *semver.Version {
	return semver.MustParse("v0.2.0")
}
