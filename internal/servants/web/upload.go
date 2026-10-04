package web

import (
	"context"
	"errors"
	"fmt"

	"github.com/BZYA-Community/WebsiteCore/internal/application/media"
	"github.com/BZYA-Community/WebsiteCore/internal/model/web"
	"github.com/sirupsen/logrus"
)

func uploadContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func mediaUploadError(err error, outputLimit int64) error {
	switch {
	case errors.Is(err, media.ErrTooLarge):
		return web.ErrFileInvalidSize.WithDetails(fmt.Sprintf("文件不得超过%dMB", outputLimit>>20))
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		return web.ErrFileUploadFailed.WithDetails("文件处理超时或已取消")
	default:
		logrus.Warnf("media validation failed: %v", err)
		return web.ErrFileInvalidExt.WithDetails("文件损坏或不符合上传要求：图片须为1920px内WebP，视频须为1080p内H.264 MP4、29.97帧")
	}
}
