package conf

import "fmt"

type UploadConf struct {
	AttachmentMaxBytes       int64 `mapstructure:"attachment_max_bytes" json:"attachment_max_bytes"`
	CourseAttachmentMaxBytes int64 `mapstructure:"course_attachment_max_bytes" json:"course_attachment_max_bytes"`
	CourseResourceMaxBytes   int64 `mapstructure:"course_resource_max_bytes" json:"course_resource_max_bytes"`
	VideoInputMaxBytes       int64 `mapstructure:"video_input_max_bytes" json:"video_input_max_bytes"`
	CredentialTTLSeconds     int64 `mapstructure:"credential_ttl_seconds" json:"-"`
}

var UploadSetting *UploadConf

func UploadLimits() UploadConf {
	if UploadSetting != nil {
		return *UploadSetting
	}
	return UploadConf{15 << 20, 50 << 20, 2 << 30, 30 << 20, 3600}
}

func (c UploadConf) Validate() error {
	if c.AttachmentMaxBytes < 1 || c.AttachmentMaxBytes > 15<<20 || c.CourseAttachmentMaxBytes < 1 || c.CourseAttachmentMaxBytes > 50<<20 || c.CourseResourceMaxBytes < 1 || c.CourseResourceMaxBytes > 2<<30 || c.VideoInputMaxBytes < 1 || c.VideoInputMaxBytes > 30<<20 || c.CredentialTTLSeconds < 60 || c.CredentialTTLSeconds > 86400 {
		return fmt.Errorf("uploads limits must be positive and cannot exceed 15 MiB / 50 MiB / 2 GiB / 30 MiB; credential TTL must be 60..86400 seconds")
	}
	return nil
}
