package web

import (
	"reflect"
	"testing"

	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
)

func TestRemovedLessonObjectURLs(t *testing.T) {
	lesson := func(video string, attachments ...string) *ms.CourseLesson {
		row := &ms.CourseLesson{VideoURL: video}
		for _, url := range attachments {
			row.Attachments = append(row.Attachments, &ms.CourseLessonAttachment{URL: url})
		}
		return row
	}
	tests := []struct {
		name string
		old  []*ms.CourseLesson
		new  []*ms.CourseLesson
		want map[string]struct{}
	}{
		{
			name: "deleted lesson removes its video and attachment",
			old:  []*ms.CourseLesson{lesson("old-video", "old-file")},
			want: map[string]struct{}{"old-video": {}, "old-file": {}},
		},
		{
			name: "object retained elsewhere in the course is kept",
			old:  []*ms.CourseLesson{lesson("shared-video", "shared-file")},
			new:  []*ms.CourseLesson{lesson("shared-video", "shared-file")},
			want: map[string]struct{}{},
		},
		{
			name: "replacement removes old objects and keeps new objects",
			old:  []*ms.CourseLesson{lesson("old-video", "old-file")},
			new:  []*ms.CourseLesson{lesson("new-video", "new-file")},
			want: map[string]struct{}{"old-video": {}, "old-file": {}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := removedLessonObjectURLs(tt.old, tt.new); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("removedLessonObjectURLs() = %#v, want %#v", got, tt.want)
			}
		})
	}
}
