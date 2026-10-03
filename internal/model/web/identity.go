package web

type MemberAccessReq struct {
	BaseInfo       `json:"-" binding:"-"`
	UserID         int64  `json:"user_id" binding:"required"`
	MemberIdentity string `json:"member_identity" binding:"required,oneof=student teacher"`
	IsMentor       bool   `json:"is_mentor"`
	IsAuditor      bool   `json:"is_auditor"`
}

type CreateAdminReq struct {
	BaseInfo          `json:"-" binding:"-"`
	Username          string `json:"username" binding:"required"`
	TemporaryPassword string `json:"temporary_password" binding:"required"`
}
