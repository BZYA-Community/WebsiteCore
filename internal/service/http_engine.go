package service

import (
	"net/http"
	"time"

	sentrygin "github.com/getsentry/sentry-go/gin"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// httpEngineOptions selects Gin defaults or the JSON endpoint policy with
// CORS and JSON routing errors, plus optional Sentry reporting.
type httpEngineOptions struct {
	API    bool
	Sentry bool
}

func newHTTPEngine(options httpEngineOptions) *gin.Engine {
	e := gin.New()
	e.Use(gin.Logger(), gin.Recovery())
	if options.API {
		e.HandleMethodNotAllowed = true
		config := cors.DefaultConfig()
		config.AllowAllOrigins = true
		config.AddAllowHeaders("Authorization")
		e.Use(cors.New(config))
	}
	if options.Sentry {
		e.Use(sentrygin.New(sentrygin.Options{Repanic: true}))
	}
	if options.API {
		e.NoRoute(func(c *gin.Context) {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "Not Found"})
		})
		e.NoMethod(func(c *gin.Context) {
			c.JSON(http.StatusMethodNotAllowed, gin.H{"code": 405, "msg": "Method Not Allowed"})
		})
	}
	return e
}

// httpSettings accepts the existing bootstrap config without exposing its
// concrete type. The first module registered at an address owns its policy.
type httpSettings interface {
	Addr() string
	GetReadTimeout() time.Duration
	GetWriteTimeout() time.Duration
}

func sharedHTTPServer(settings httpSettings, engine func() *gin.Engine) *httpServer {
	addr := settings.Addr()
	return httpServers.from(addr, func() *httpServer {
		e := engine()
		return &httpServer{
			baseServer: newBaseServe(),
			e:          e,
			server: &http.Server{
				Addr: addr, Handler: e,
				ReadTimeout:    settings.GetReadTimeout(),
				WriteTimeout:   settings.GetWriteTimeout(),
				MaxHeaderBytes: 1 << 20,
			},
		}
	})
}
