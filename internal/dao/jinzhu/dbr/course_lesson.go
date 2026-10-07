package dbr

// CourseLesson metadata is separate from the course so saving a course cannot
// erase lessons that the editor failed to load.
type CourseLesson struct {
	*Model
	CourseID    int64                     `json:"course_id"`
	Title       string                    `json:"title"`
	Intro       string                    `json:"intro"`
	Sort        int                       `json:"sort"`
	Attachments []*CourseLessonAttachment `gorm:"-" json:"attachments"`
}

// CourseLessonAttachment references a verified upload; it never exposes a key.
type CourseLessonAttachment struct {
	ID           int64  `gorm:"primaryKey" json:"id"`
	LessonID     int64  `json:"-"`
	AttachmentID int64  `json:"attachment_id"`
	Name         string `json:"name"`
	Kind         string `json:"kind"`
	Sort         int    `json:"-"`
	FileSize     int64  `gorm:"-" json:"file_size"`
	MimeType     string `gorm:"-" json:"mime_type"`
}
