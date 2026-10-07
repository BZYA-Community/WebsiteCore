package core

import (
	"io"

	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
)

type CourseUploadService interface {
	StartCourseUpload(actor *ms.User, name, kind, mimeType string, size int64) (*ms.Attachment, error)
	WriteCourseUpload(id int64, contentType string, reader io.Reader, oss ObjectStorageService) error
	CompleteCourseUpload(actor *ms.User, id int64, oss ObjectStorageService) (*ms.Attachment, error)
	ExpireCourseUploads(oss ObjectStorageService) error
}
