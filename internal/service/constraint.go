//go:build constraint

package service

var (
	_ server = (*httpServer)(nil)

	_ Service = (*frontendWebService)(nil)
	_ Service = (*metricsService)(nil)
	_ Service = (*pprofService)(nil)
	_ Service = (*webService)(nil)
)
