package storage

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/sirupsen/logrus"
)

func (s *localossServant) InspectObject(key string) (*core.ObjectMetadata, error) {
	f, err := OpenLocalObject(s.savePath, key)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	header, err := io.ReadAll(io.LimitReader(f, 512))
	if err != nil {
		return nil, err
	}
	return &core.ObjectMetadata{Size: info.Size(), ContentType: http.DetectContentType(header), Header: header,
		ETag: localObjectETag(info)}, nil
}

func localObjectETag(info os.FileInfo) string {
	return fmt.Sprintf("%d:%d", info.Size(), info.ModTime().UnixNano())
}

func (s *localossServant) PromoteObject(source, destination, etag string) error {
	if source == destination || etag == "" {
		return fmt.Errorf("invalid object promotion")
	}
	f, err := OpenLocalObject(s.savePath, source)
	if err != nil {
		return err
	}
	defer f.Close()
	before, err := f.Stat()
	if err != nil {
		return err
	}
	if localObjectETag(before) != etag {
		return fmt.Errorf("upload changed during verification")
	}
	src, err := jailPath(s.savePath, source)
	if err != nil {
		return err
	}
	dst, err := jailPath(s.savePath, destination)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		return err
	}
	// Upload writes publish immutable, closed files. Linking is atomic, refuses
	// overwrite, and does not read a multi-gigabyte object into the process.
	linkErr := os.Link(src, dst)
	after, err := os.Stat(dst)
	if err != nil {
		return errors.Join(linkErr, err)
	}
	if !os.SameFile(before, after) || localObjectETag(after) != etag {
		if linkErr == nil {
			_ = os.Remove(dst)
		}
		return fmt.Errorf("promotion destination does not match verified source")
	}
	return nil
}

func (s *aliossServant) InspectObject(key string) (*core.ObjectMetadata, error) {
	meta, err := s.bucket.GetObjectDetailedMeta(key)
	if err != nil {
		return nil, err
	}
	size, err := strconv.ParseInt(meta.Get("Content-Length"), 10, 64)
	if err != nil || size < 1 {
		return nil, fmt.Errorf("invalid OSS object size")
	}
	etag := meta.Get("ETag")
	if etag == "" || meta.Get("X-Oss-Object-Type") == "Symlink" {
		return nil, fmt.Errorf("invalid OSS source object")
	}
	body, err := s.bucket.GetObject(key, oss.Range(0, min(size, 512)-1), oss.IfMatch(etag))
	if err != nil {
		return nil, err
	}
	defer body.Close()
	header, err := io.ReadAll(io.LimitReader(body, 512))
	if err != nil {
		return nil, err
	}
	if int64(len(header)) != min(size, 512) {
		return nil, io.ErrUnexpectedEOF
	}
	return &core.ObjectMetadata{Size: size, ContentType: meta.Get("Content-Type"), Header: header, ETag: etag}, nil
}

const promotionMetadata = "verified-source"

func (s *aliossServant) promotedObjectMatches(key, sourceID string, size int64) (bool, error) {
	meta, err := s.bucket.GetObjectDetailedMeta(key)
	if err != nil {
		var serviceErr oss.ServiceError
		if errors.As(err, &serviceErr) && serviceErr.StatusCode == http.StatusNotFound {
			return false, nil
		}
		return false, err
	}
	if meta.Get("X-Oss-Meta-"+promotionMetadata) != sourceID || meta.Get("Content-Length") != strconv.FormatInt(size, 10) {
		return false, fmt.Errorf("promotion destination already belongs to a different source")
	}
	acl, err := s.bucket.GetObjectACL(key)
	if err != nil {
		return false, err
	}
	if acl.ACL != string(oss.ACLPrivate) {
		return false, fmt.Errorf("promoted object must have private ACL")
	}
	return true, nil
}

func (s *aliossServant) PromoteObject(source, destination, etag string) error {
	if source == destination || etag == "" {
		return fmt.Errorf("invalid object promotion")
	}
	meta, err := s.bucket.GetObjectDetailedMeta(source, oss.IfMatch(etag))
	if err != nil {
		return err
	}
	size, err := strconv.ParseInt(meta.Get("Content-Length"), 10, 64)
	if err != nil || size <= 0 || meta.Get("ETag") != etag {
		return fmt.Errorf("upload changed during verification")
	}
	sourceID := fmt.Sprintf("%x", sha256.Sum256([]byte(source+"\x00"+etag)))
	if exists, err := s.promotedObjectMatches(destination, sourceID, size); err != nil || exists {
		return err
	}
	options := []oss.Option{oss.ObjectACL(oss.ACLPrivate), oss.ContentType(meta.Get("Content-Type")), oss.Meta(promotionMetadata, sourceID)}
	if !strings.HasPrefix(meta.Get("Content-Type"), "video/") {
		options = append(options, oss.ContentDisposition("attachment"))
	}
	completed := false
	if size <= 1<<30 {
		_, err = s.bucket.CopyObject(source, destination, append(options, oss.MetadataDirective(oss.MetaReplace), oss.CopySourceIfMatch(etag), oss.ForbidOverWrite(true))...)
	} else {
		// Conservatively use multipart above 1 GiB across OSS copy variants.
		// Each source range is conditional; no resource bytes pass through Go.
		var upload oss.InitiateMultipartUploadResult
		upload, err = s.bucket.InitiateMultipartUpload(destination, options...)
		if err != nil {
			return err
		}
		defer func() {
			if !completed {
				if abortErr := s.bucket.AbortMultipartUpload(upload); abortErr != nil {
					logrus.WithError(abortErr).Warn("could not abort failed OSS multipart copy")
				}
			}
		}()
		const partSize int64 = 128 << 20
		parts := make([]oss.UploadPart, 0, (size+partSize-1)/partSize)
		for offset := int64(0); offset < size; offset += partSize {
			part, copyErr := s.bucket.UploadPartCopy(upload, s.bucket.BucketName, source, offset, min(partSize, size-offset), len(parts)+1, oss.CopySourceIfMatch(etag))
			if copyErr != nil {
				return copyErr
			}
			parts = append(parts, part)
		}
		_, err = s.bucket.CompleteMultipartUpload(upload, parts, oss.ForbidOverWrite(true))
	}
	// A timeout may occur after OSS committed. Accept only our matching private
	// object; never overwrite a destination or trust an unrelated existing key.
	exists, verifyErr := s.promotedObjectMatches(destination, sourceID, size)
	if verifyErr != nil {
		return errors.Join(err, verifyErr)
	}
	if exists {
		completed = true
		return nil
	}
	if err != nil {
		return err
	}
	return fmt.Errorf("promotion did not publish the verified object")
}
