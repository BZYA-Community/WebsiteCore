package core

import "github.com/BZYA-Community/WebsiteCore/internal/core/ms"

type IdentityService interface {
	LoadUserIdentities(users ...*ms.User) error
	ListIdentityGroups() ([]*ms.IdentityGroup, error)
	SaveIdentityGroup(actor *ms.User, group *ms.IdentityGroup) (*ms.IdentityGroup, error)
	DeleteIdentityGroup(actor *ms.User, id int64) error
	SetUserIdentityGroups(actor *ms.User, userID int64, groupIDs []int64) error
	SetManagedUserStatus(actor *ms.User, userID int64, status int) error
	DeleteManagedUser(actor *ms.User, userID int64) error
	ListIdentityLogs(offset, limit int) ([]*ms.IdentityOperationLog, int64, error)
}
