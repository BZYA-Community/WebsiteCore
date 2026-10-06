package dbr

// Codes are keyed to the authenticated account and stored only as keyed hashes.
type ContactVerification struct {
	ID        int64 `gorm:"primaryKey"`
	UserID    int64 `gorm:"index"`
	Mode      string
	Address   string `gorm:"index"`
	IPHash    string `gorm:"index"`
	CodeHash  string `json:"-"`
	CreatedOn int64  `gorm:"index"`
	ExpiresOn int64
	Attempts  int
	Used      bool
	Delivered bool
}
