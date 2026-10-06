package ms

import "github.com/BZYA-Community/WebsiteCore/internal/dao/jinzhu/dbr"

type ReviewTask = dbr.ReviewTask
type ReviewTaskEvent = dbr.ReviewTaskEvent
type ReviewerStatistics = dbr.ReviewerStatistics

const (
	ReviewPost           = dbr.ReviewPost
	ReviewComment        = dbr.ReviewComment
	ReviewReply          = dbr.ReviewReply
	ReviewCourseQuestion = dbr.ReviewCourseQuestion
	ReviewCourseAnswer   = dbr.ReviewCourseAnswer
	ReviewNickname       = dbr.ReviewNickname
	ReviewAvatar         = dbr.ReviewAvatar
	ReviewPending        = dbr.ReviewPending
	ReviewCompleted      = dbr.ReviewCompleted
	ReviewCancelled      = dbr.ReviewCancelled
)
