package main

import (
	"context"

	"github.com/pokt-network/poktroll/pkg/polylog"

	"github.com/buildwithgrove/path-external-auth-server/metrics"
)

// TODO_TECHDEBT(@adshmh): Support configurable pprof server address/port.
const (
	// pprofAddr is the address at which pprof server will be listening.
	// NOTE: This address was selected based on the example here:
	// https://pkg.go.dev/net/http/pprof
	pprofAddr = ":6060"
)

// setupMetricsServer initializes and starts the Prometheus metrics server at the supplied address.
func setupMetricsServer(logger polylog.Logger, addr, version string) error {
	return metrics.ServeMetrics(logger, addr, version)
}

// setupPprofServer starts the pprof server at the supplied address.
func setupPprofServer(ctx context.Context, logger polylog.Logger, addr string) {
	metrics.ServePprof(ctx, logger, addr)
}
