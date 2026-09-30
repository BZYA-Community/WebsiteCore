// Package httpx owns request binding, authenticated request context, and JSON
// response rendering. It never constructs persistence or application modules.
package httpx

import (
	"fmt"

	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/pkg/app"
	"github.com/BZYA-Community/WebsiteCore/pkg/xerror"
	"github.com/alimy/mir/v5"
	"github.com/cockroachdb/errors"
	"github.com/getsentry/sentry-go"
	sentrygin "github.com/getsentry/sentry-go/gin"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

// Servant provides the interface expected by generated HTTP adapters.
type Servant struct{ bind func(*gin.Context, any) error }

func New(sentryEnabled bool) *Servant {
	bind := bindAny
	if sentryEnabled {
		bind = bindAnySentry
	}
	return &Servant{bind: bind}
}
func (s *Servant) Bind(c *gin.Context, obj any) error { return s.bind(c, obj) }

// BindJson preserves the existing ShouldBind behavior, including form binding.
func (s *Servant) BindJson(c *gin.Context, obj any) error     { return s.Bind(c, obj) }
func (s *Servant) Render(c *gin.Context, data any, err error) { Render(c, data, err) }

type SentryHubSetter interface {
	SetSentryHub(hub *sentry.Hub)
}

type UserSetter interface {
	SetUser(*ms.User)
}

type UserIdSetter interface {
	SetUserId(int64)
}

type PageInfoSetter interface {
	SetPageInfo(page, pageSize int)
}

func UserFrom(c *gin.Context) (*ms.User, bool) {
	if u, exists := c.Get("USER"); exists {
		user, ok := u.(*ms.User)
		return user, ok
	}
	return nil, false
}

func UserIdFrom(c *gin.Context) (int64, bool) {
	if uid, exists := c.Get("UID"); exists {
		v, ok := uid.(int64)
		return v, ok
	}
	return -1, false
}

func UserNameFrom(c *gin.Context) (string, bool) {
	if username, exists := c.Get("USERNAME"); exists {
		v, ok := username.(string)
		return v, ok
	}
	return "", false
}

func bindAny(c *gin.Context, obj any) error {
	var errs xerror.ValidErrors
	err := c.ShouldBind(obj)
	if err != nil {
		// 逐字段收集入参校验错误明细，便于客户端定位具体出错字段
		if validationErrs, ok := err.(validator.ValidationErrors); ok {
			for _, fieldErr := range validationErrs {
				errs = append(errs, &xerror.ValidError{
					Message: fmt.Sprintf("字段 %s 校验失败: %s", fieldErr.Field(), fieldErr.Tag()),
				})
			}
		} else {
			errs = append(errs, &xerror.ValidError{Message: err.Error()})
		}
		return mir.NewError(xerror.InvalidParams.StatusCode(), xerror.InvalidParams.WithDetails(errs.Errors()...))
	}
	hydrate(c, obj)
	return nil
}

func bindAnySentry(c *gin.Context, obj any) error {
	hub := sentrygin.GetHubFromContext(c)
	var errs xerror.ValidErrors
	err := c.ShouldBind(obj)
	if err != nil {
		xerr := mir.NewError(xerror.InvalidParams.StatusCode(), xerror.InvalidParams.WithDetails(errs.Error()))
		if hub != nil {
			hub.CaptureException(errors.Wrap(xerr, "bind object"))
		}
		return xerr
	}
	// setup sentry hub if needed
	if setter, ok := obj.(SentryHubSetter); ok && hub != nil {
		setter.SetSentryHub(hub)
	}
	hydrate(c, obj)
	return nil
}

func hydrate(c *gin.Context, obj any) {
	// setup *core.User if needed
	if setter, ok := obj.(UserSetter); ok {
		user, _ := UserFrom(c)
		setter.SetUser(user)
	}
	// setup UserId if needed
	if setter, ok := obj.(UserIdSetter); ok {
		uid, _ := UserIdFrom(c)
		setter.SetUserId(uid)
	}
	// setup PageInfo if needed
	if setter, ok := obj.(PageInfoSetter); ok {
		page, pageSize := app.GetPageInfo(c)
		setter.SetPageInfo(page, pageSize)
	}
}
