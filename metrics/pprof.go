package metrics

import (
	"context"
	"net/http"
	"net/http/pprof"

	"github.com/pokt-network/poktroll/pkg/polylog"
)

// ServePprof starts a pprof server on the given address.
func ServePprof(ctx context.Context, logger polylog.Logger, addr string) {
	pprofMux := http.NewServeMux()
	pprofMux.HandleFunc("/debug/pprof/", pprof.Index)
	pprofMux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	pprofMux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	pprofMux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	pprofMux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	server := &http.Server{
		Addr:    addr,
		Handler: pprofMux,
	}

	// Start the server in a new goroutine
	go func() {
		logger.Info().Str("pprof_addr", addr).Msg("🔬 Starting pprof server for runtime debugging")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error().Err(err).Str("pprof_addr", addr).Msg("pprof server failed")
		}
	}()

	// Handle graceful shutdown
	go func() {
		<-ctx.Done()
		logger.Info().Str("pprof_addr", addr).Msg("Stopping pprof server")
		if err := server.Shutdown(ctx); err != nil {
			logger.Error().Err(err).Msg("Error stopping pprof server")
		}
	}()
}
