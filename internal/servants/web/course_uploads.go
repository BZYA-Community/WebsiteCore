package web

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/BZYA-Community/WebsiteCore/internal/conf"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/storage"
	"github.com/BZYA-Community/WebsiteCore/internal/model/web"
	"github.com/alimy/tryst/cfg"
	"github.com/gin-gonic/gin"
)

func (s *courseAdminSrv) CourseUploadInit(req *web.CourseUploadInitReq) (*web.CourseUploadInitResp, error) {
	a, err := s.Ds.StartCourseUpload(req.User, req.Name, req.Kind, req.MimeType, req.Size)
	if err != nil {
		return nil, courseMutationError(err, web.ErrFileUploadFailed)
	}
	return courseUploadCredential(a, cfg.If("AliOSS"))
}

func courseUploadCredential(a *ms.Attachment, ali bool) (*web.CourseUploadInitResp, error) {
	resp := &web.CourseUploadInitResp{AttachmentID: a.ID, Mode: "direct", Method: "PUT", ExpiresOn: a.UploadExpiresOn}
	if !ali {
		path := fmt.Sprintf("/v1/course/upload/%d", a.ID)
		resp.UploadURL = fmt.Sprintf("%s?expired=%d&sign=%s", path, a.UploadExpiresOn, storage.LocalOSSSign("PUT:"+path, a.UploadExpiresOn))
		return resp, nil
	}
	settings := conf.AliOSSSetting
	conditions := []any{map[string]string{"bucket": settings.Bucket}, []any{"content-length-range", a.FileSize, a.FileSize}}
	fields := map[string]string{"key": a.Content, "Content-Type": a.MimeType, "success_action_status": "200", "x-oss-object-acl": "private", "x-oss-forbid-overwrite": "true"}
	for key, value := range fields {
		conditions = append(conditions, []string{"eq", "$" + key, value})
	}
	policyJSON, err := json.Marshal(map[string]any{"expiration": time.Unix(a.UploadExpiresOn, 0).UTC().Format(time.RFC3339), "conditions": conditions})
	if err != nil {
		return nil, err
	}
	policy := base64.StdEncoding.EncodeToString(policyJSON)
	mac := hmac.New(sha1.New, []byte(settings.AccessKeySecret))
	mac.Write([]byte(policy))
	fields["policy"], fields["Signature"], fields["OSSAccessKeyId"] = policy, base64.StdEncoding.EncodeToString(mac.Sum(nil)), settings.AccessKeyID
	endpoint := strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(settings.Endpoint, "https://"), "http://"), "/")
	resp.Method, resp.Fields, resp.UploadURL = "POST", fields, "https://"+settings.Bucket+"."+endpoint
	return resp, nil
}

func (s *courseAdminSrv) CourseUploadComplete(req *web.CourseUploadCompleteReq) (*web.CourseUploadCompleteResp, error) {
	a, err := s.Ds.CompleteCourseUpload(req.User, req.AttachmentID, s.oss)
	if err != nil {
		return nil, courseMutationError(err, web.ErrFileUploadFailed)
	}
	return &web.CourseUploadCompleteResp{AttachmentID: a.ID, Name: a.Name, FileSize: a.FileSize, MimeType: a.MimeType, Kind: strings.TrimPrefix(a.Purpose, "course_")}, nil
}

// This route uses a short-lived, method-bound upload capability issued only to
// a current course uploader. The DAO rechecks that account before accepting bytes.
func putLocalCourseUpload(c *gin.Context) {
	expires, err := strconv.ParseInt(c.Query("expired"), 10, 64)
	id, idErr := strconv.ParseInt(c.Param("id"), 10, 64)
	if cfg.If("AliOSS") || err != nil || idErr != nil || id <= 0 || !conf.CoursesEnabled() || !storage.VerifyLocalOSSSign("PUT:"+c.Request.URL.Path, expires, c.Query("sign")) {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	controller := http.NewResponseController(c.Writer)
	deadline := time.Unix(expires, 0)
	if err := controller.SetReadDeadline(deadline); err != nil {
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}
	if err := controller.SetWriteDeadline(deadline); err != nil {
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, conf.UploadLimits().CourseResourceMaxBytes+1)
	if err := _ds.WriteCourseUpload(id, c.GetHeader("Content-Type"), c.Request.Body, _oss); err != nil {
		c.String(http.StatusBadRequest, "Course upload was rejected")
		return
	}
	c.Status(http.StatusNoContent)
}
