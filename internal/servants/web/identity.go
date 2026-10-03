package web

import (
	"errors"

	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/jinzhu/dbr"
	"github.com/BZYA-Community/WebsiteCore/internal/infra/avatar"
	"github.com/BZYA-Community/WebsiteCore/internal/model/web"
	"github.com/BZYA-Community/WebsiteCore/pkg/xerror"
)

// Only domain errors are exposed. Database/internal errors never reach clients.
func accessError(err error) *xerror.Error {
	switch {
	case errors.Is(err, dbr.ErrPermission):
		return web.ErrNoPermission
	case errors.Is(err, dbr.ErrTeacherInUse):
		return web.ErrTeacherInUse
	case errors.Is(err, dbr.ErrWhisperPhone):
		return web.ErrWhisperNeedPhone
	case errors.Is(err, dbr.ErrWhisperPending):
		return web.ErrWhisperOnePending
	case errors.Is(err, dbr.ErrWhisperIdentity):
		return web.ErrWhisperIdentityDenied
	case errors.Is(err, dbr.ErrWhisperBlocked):
		return web.ErrWhisperBlocked
	default:
		return xerror.ServerError
	}
}

func (s *adminSrv) expireManagedUser(id int64) {
	if user, err := s.Ds.GetUserByID(id); err == nil {
		onChangeUsernameEvent(id, user.Username)
	}
}

func (s *adminSrv) ChangeMemberAccess(req *web.MemberAccessReq) error {
	if req.User == nil {
		return web.ErrNoPermission
	}
	if err := s.Ds.ChangeMemberAccess(req.User.ID, req.UserID, req.MemberIdentity, req.IsMentor, req.IsAuditor); err != nil {
		return accessError(err)
	}
	s.expireManagedUser(req.UserID)
	return nil
}

func (s *adminSrv) CreateAdmin(req *web.CreateAdminReq) (*web.RegisterResp, error) {
	if !req.User.CanCreateAdmin() {
		return nil, web.ErrNoPermission
	}
	pub := &pubSrv{DaoServant: s.DaoServant}
	if err := pub.validUsername(req.Username); err != nil {
		return nil, err
	}
	if err := checkPassword(req.TemporaryPassword); err != nil {
		return nil, err
	}
	password, salt := encryptPasswordAndSalt(req.TemporaryPassword)
	avatarURL, err := avatar.Generate(req.Username)
	if err != nil {
		return nil, xerror.ServerError
	}
	user, err := s.Ds.CreateAdmin(req.User.ID, &ms.User{
		Username: req.Username, Nickname: req.Username, Password: password,
		Salt: salt, Avatar: avatarURL,
	})
	if err != nil {
		return nil, accessError(err)
	}
	return &web.RegisterResp{UserId: user.ID, Username: user.Username}, nil
}

func (s *chatSrv) SetWhisperBlock(req *web.WhisperBlockReq) error {
	if req.User == nil {
		return web.ErrNoPermission
	}
	if err := s.Ds.SetWhisperBlock(req.User.ID, req.UserID, req.Blocked); err != nil {
		return accessError(err)
	}
	return nil
}

func (s *courseAdminSrv) CourseTeachers(req *web.CourseTeachersReq) (*web.CourseTeachersResp, error) {
	if !req.User.CanManageUsers() {
		return nil, web.ErrNoPermission
	}
	users, err := s.Ds.ListCourseTeachers(req.Keyword)
	if err != nil {
		return nil, xerror.ServerError
	}
	teachers := make([]*ms.UserFormated, 0, len(users))
	for _, user := range users {
		teachers = append(teachers, user.Format())
	}
	return &web.CourseTeachersResp{Teachers: teachers}, nil
}

func (s *auditSrv) auditCommentAuthor(kind int, id int64) (int64, error) {
	switch kind {
	case 0:
		v, err := s.Ds.GetCommentByID(id)
		if err == nil && v != nil {
			return v.UserID, nil
		}
	case 1:
		v, err := s.Ds.GetCommentReplyByID(id)
		if err == nil && v != nil {
			return v.UserID, nil
		}
	case 2:
		v, err := s.Ds.GetCourseCommentByID(id)
		if err == nil && v != nil {
			return v.UserID, nil
		}
	case 3:
		v, err := s.Ds.GetCourseCommentReplyByID(id)
		if err == nil && v != nil {
			return v.UserID, nil
		}
	}
	return 0, web.ErrAuditCommentFailed
}
