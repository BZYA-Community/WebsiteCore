package core

import (
	"errors"
	"time"

	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
)

var ErrReviewStale = errors.New("review task changed; refresh before deciding")

// ReviewDecision contains committed changes for cache/search/notification work.
// Database status, counters, task history and the audit log commit together.
type ReviewDecision struct {
	Task      *ms.ReviewTask
	OldStatus int
	NewStatus int
	ParentID  int64
	OldValue  string
	NewValue  string
}

type ProfileSubmission struct {
	Pending  bool
	Previous string
}

type ReviewService interface {
	CreatePostWithContents(post *ms.Post, contents []*ms.PostContent) (*ms.Post, error)
	CreateCommentWithContents(comment *ms.Comment, contents []*ms.CommentContent) (*ms.Comment, error)
	CreateCourseQuestionWithContents(question *ms.CourseComment, contents []*ms.CourseCommentContent) (*ms.CourseComment, error)
	SubmitProfileReview(user *ms.User, kind, value string) (*ProfileSubmission, error)
	SetPostVisibilityForReview(actor *ms.User, postID int64, visibility ms.PostVisibleT) (*ms.Post, error)
	ReconcileReviewTasks(deadline time.Duration) error
	ListReviewTasks(actor *ms.User, kind, state string, offset, limit int) ([]*ms.ReviewTask, int64, error)
	ReviewTaskForTarget(actor *ms.User, kind string, targetID int64) (*ms.ReviewTask, error)
	DecideReview(actor *ms.User, taskID, revision int64, action, reason string) (*ReviewDecision, error)
	ListReviewTaskEvents(actor *ms.User, taskID int64) ([]*ms.ReviewTaskEvent, error)
	ReviewerTimeoutStatistics(actor *ms.User) ([]*ms.ReviewerStatistics, error)
}
