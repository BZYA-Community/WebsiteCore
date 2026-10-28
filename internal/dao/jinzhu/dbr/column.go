package dbr

const (
	ColumnArticle                  = "article"
	ColumnCollection               = "collection"
	ReviewColumn                   = "column"
	ReviewColumnComment            = "column_comment"
	ColumnWithdrawn     PostAuditT = 3
)

// Column holds only the approved public content. Submissions never overwrite it.
type Column struct {
	*Model
	UserID             int64             `json:"user_id"`
	Kind               string            `json:"kind"`
	Title              string            `json:"title"`
	Summary            string            `json:"summary"`
	Body               string            `json:"body"`
	Cover              string            `json:"cover"`
	Attachments        []*ColumnContent  `gorm:"serializer:json;type:jsonb" json:"attachments"`
	PublishedOn        int64             `json:"published_on"`
	UpdatedOn          int64             `json:"updated_on"`
	PublicSubmissionID int64             `json:"public_submission_id"`
	SubmissionID       int64             `json:"submission_id"`
	Official           bool              `json:"official"`
	Locked             bool              `json:"locked"`
	Blocked            bool              `json:"blocked"`
	Curated            bool              `json:"curated"`
	User               *UserFormated     `gorm:"-" json:"user,omitempty"`
	Submission         *ColumnSubmission `gorm:"-" json:"submission,omitempty"`
}

type ColumnSubmission struct {
	*Model
	ColumnID    int64            `json:"column_id"`
	UserID      int64            `json:"user_id"`
	Title       string           `json:"title"`
	Summary     string           `json:"summary"`
	Body        string           `json:"body"`
	Cover       string           `json:"cover"`
	Attachments []*ColumnContent `gorm:"serializer:json;type:jsonb" json:"attachments"`
	AuditStatus PostAuditT       `json:"audit_status"`
	Reason      string           `json:"reason"`
}

type ColumnContent struct {
	Content string       `json:"content"`
	Type    PostContentT `json:"type"`
	Sort    int64        `json:"sort"`
}

type ColumnEntry struct {
	CollectionID int64 `gorm:"primaryKey" json:"collection_id"`
	ArticleID    int64 `gorm:"primaryKey" json:"article_id"`
	Position     int   `json:"position"`
}

// A parent_id of zero denotes a top-level comment. Replies remain flat.
type ColumnComment struct {
	*Model
	ColumnID    int64            `json:"column_id"`
	ParentID    int64            `json:"parent_id"`
	UserID      int64            `json:"user_id"`
	AtUserID    int64            `json:"at_user_id"`
	Content     string           `json:"content"`
	Contents    []*ColumnContent `gorm:"serializer:json;type:jsonb" json:"contents"`
	Mentions    []string         `gorm:"serializer:json;type:jsonb" json:"-"`
	IPLoc       string           `json:"ip_loc"`
	AuditStatus PostAuditT       `json:"audit_status"`
	Reason      string           `json:"reason"`
	Featured    bool             `json:"featured"`
	Likes       int64            `json:"likes"`
	Dislikes    int64            `json:"-"`
	User        *UserFormated    `gorm:"-" json:"user,omitempty"`
	AtUser      *UserFormated    `gorm:"-" json:"at_user,omitempty"`
	Reaction    int              `gorm:"-" json:"reaction"`
	Replies     []*ColumnComment `gorm:"-" json:"replies"`
}

type ColumnReaction struct {
	CommentID int64 `gorm:"primaryKey"`
	UserID    int64 `gorm:"primaryKey"`
	Value     int
}
