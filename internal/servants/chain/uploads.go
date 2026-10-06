package chain

import (
	"net/http"

	"github.com/BZYA-Community/WebsiteCore/internal/conf"
	"github.com/gin-gonic/gin"
)

func UploadBodyLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.FullPath() == "/v1/attachment" && c.Request.Method == http.MethodPost {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, conf.UploadLimits().AttachmentMaxBytes+(1<<20))
			defer func() {
				if c.Request.MultipartForm != nil {
					_ = c.Request.MultipartForm.RemoveAll()
				}
			}()
		}
		c.Next()
	}
}
