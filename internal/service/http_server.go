// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package service

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// httpShutdownTimeout http停机超时兜底，避免挂起请求导致进程永远退不出去
const httpShutdownTimeout = 10 * time.Second

// httpServer wraper for gin.engine and http.Server
type httpServer struct {
	*baseServer

	e      *gin.Engine
	server *http.Server
}

func (s *httpServer) start() error {
	return s.server.ListenAndServe()
}

func (s *httpServer) stop() error {
	// 带超时的优雅停机，超时后强制关闭，避免挂起请求阻塞进程退出
	ctx, cancel := context.WithTimeout(context.Background(), httpShutdownTimeout)
	defer cancel()
	if err := s.server.Shutdown(ctx); err != nil {
		logrus.Errorf("httpServer shutdown occurs error: %v, force close server", err)
		if closeErr := s.server.Close(); closeErr != nil {
			logrus.Errorf("httpServer force close occurs error: %v", closeErr)
		}
		return err
	}
	return nil
}
