package ms

import "github.com/BZYA-Community/WebsiteCore/internal/dao/jinzhu/dbr"

type Column = dbr.Column
type ColumnSubmission = dbr.ColumnSubmission
type ColumnContent = dbr.ColumnContent
type ColumnEntry = dbr.ColumnEntry
type ColumnComment = dbr.ColumnComment
type ColumnReaction = dbr.ColumnReaction

const (
	ColumnArticle       = dbr.ColumnArticle
	ColumnCollection    = dbr.ColumnCollection
	ReviewColumn        = dbr.ReviewColumn
	ReviewColumnComment = dbr.ReviewColumnComment
	ColumnWithdrawn     = dbr.ColumnWithdrawn
)
