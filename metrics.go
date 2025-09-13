package main

import (
	"context"

	"github.com/pokt-network/poktroll/pkg/polylog"

	"github.com/buildwithgrove/path-external-auth-server/metrics"
)

// setupMetricsServer initializes and starts the Prometheus metrics server at the supplied address.
func setupMetricsServer(logger polylog.Logger, addr, version string) error {
	return metrics.ServeMetrics(logger, addr, version)
}

// setupPprofServer starts the pprof server at the supplied address.
func setupPprofServer(ctx context.Context, logger polylog.Logger, addr string) {
	metrics.ServePprof(ctx, logger, addr)
}
