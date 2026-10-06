package dbr

const (
	ReviewPost           = "post"
	ReviewComment        = "comment"
	ReviewReply          = "reply"
	ReviewCourseQuestion = "course_question"
	ReviewCourseAnswer   = "course_answer"
	ReviewNickname       = "nickname"
	ReviewAvatar         = "avatar"
	ReviewPending        = "pending"
	ReviewCompleted      = "completed"
	ReviewCancelled      = "cancelled"
)

// ReviewTask is a durable assignment. Revision prevents an old browser tab
// from approving a replacement nickname/avatar or a resubmitted post.
type ReviewTask struct {
	ID                 int64  `gorm:"primaryKey" json:"id"`
	Kind               string `gorm:"size:32;uniqueIndex:review_target" json:"kind"`
	TargetID           int64  `gorm:"uniqueIndex:review_target" json:"target_id"`
	AuthorID           int64  `json:"author_id"`
	Revision           int64  `json:"revision"`
	State              string `gorm:"size:16;index" json:"state"`
	Snapshot           string `gorm:"type:text" json:"-"`
	AssigneeID         int64  `gorm:"index" json:"assignee_id"`
	PreviousAssigneeID int64  `json:"-"`
	AssignedOn         int64  `json:"assigned_on"`
	DeadlineOn         int64  `gorm:"index" json:"deadline_on"`
	CompletedOn        int64  `json:"completed_on"`
	CreatedOn          int64  `json:"created_on"`
	ModifiedOn         int64  `json:"modified_on"`
}

// ReviewTaskEvent is append-only assignment/decision history. Timeout counts
// are derived from these rows, so reassignment and the recorded miss are atomic.
type ReviewTaskEvent struct {
	ID             int64  `gorm:"primaryKey" json:"id"`
	TaskID         int64  `gorm:"index" json:"task_id"`
	Revision       int64  `json:"revision"`
	Event          string `gorm:"size:32;index" json:"event"`
	ActorID        int64  `json:"actor_id"`
	FromAssigneeID int64  `gorm:"index" json:"from_assignee_id"`
	ToAssigneeID   int64  `json:"to_assignee_id"`
	Reason         string `gorm:"size:255" json:"reason"`
	CreatedOn      int64  `json:"created_on"`
}

type ReviewerStatistics struct {
	UserID       int64 `json:"user_id"`
	TimeoutCount int64 `json:"timeout_count"`
}
