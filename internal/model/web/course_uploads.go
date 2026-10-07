package web

type CourseUploadInitReq struct {
	BaseInfo `json:"-" binding:"-"`
	Name     string `json:"name" binding:"required,max=255"`
	Size     int64  `json:"size" binding:"required,gt=0"`
	MimeType string `json:"mime_type" binding:"required,max=200"`
	Kind     string `json:"kind" binding:"required,oneof=attachment resource"`
}

type CourseUploadInitResp struct {
	AttachmentID int64             `json:"attachment_id"`
	Mode         string            `json:"mode"`
	UploadURL    string            `json:"upload_url"`
	Method       string            `json:"method"`
	Fields       map[string]string `json:"fields,omitempty"`
	ExpiresOn    int64             `json:"expires_on"`
}

type CourseUploadCompleteReq struct {
	BaseInfo     `json:"-" binding:"-"`
	AttachmentID int64 `json:"attachment_id" binding:"required,gt=0"`
}

type CourseUploadCompleteResp struct {
	AttachmentID int64  `json:"attachment_id"`
	Name         string `json:"name"`
	FileSize     int64  `json:"file_size"`
	MimeType     string `json:"mime_type"`
	Kind         string `json:"kind"`
}
