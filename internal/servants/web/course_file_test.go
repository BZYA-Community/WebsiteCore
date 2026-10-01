package web

import "testing"

func TestCourseDocumentAttachmentTypes(t *testing.T) {
	tests := map[string]string{
		"application/pdf":    ".pdf",
		"application/msword": ".doc",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document":   ".docx",
		"application/vnd.ms-powerpoint":                                             ".ppt",
		"application/vnd.openxmlformats-officedocument.presentationml.presentation": ".pptx",
	}
	for mime, want := range tests {
		got, err := getFileExt(mime)
		if err != nil || got != want {
			t.Fatalf("getFileExt(%q) = %q, %v; want %q", mime, got, err, want)
		}
	}
}
