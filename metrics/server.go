package metrics

import (
	"encoding/json"
	"net/http"

	"github.com/pokt-network/poktroll/pkg/polylog"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	endpointMetrics = "/metrics"
	endpointHealth  = "/healthz"
)

// HealthResponse represents the JSON response for the health endpoint.
type HealthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
	Version string `json:"version,omitempty"`
}

// ServeMetrics starts a Prometheus metrics server with health endpoint on the given address.
func ServeMetrics(logger polylog.Logger, addr, version string) error {
	// Create a new mux to handle multiple endpoints
	mux := http.NewServeMux()

	// Add metrics endpoint
	mux.Handle(endpointMetrics, promhttp.Handler())

	// Add health endpoint
	mux.HandleFunc(endpointHealth, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		response := HealthResponse{
			Status:  "healthy",
			Service: "peas",
			Version: version,
		}

		if err := json.NewEncoder(w).Encode(response); err != nil {
			logger.Error().Err(err).Msg("Failed to encode health response")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	})

	// Start the server in a new goroutine
	go func() {
		logger.Info().Str("metrics_addr", addr).Msg("📊 Starting Prometheus metrics server with health endpoint")
		if err := http.ListenAndServe(addr, mux); err != nil {
			logger.Error().Err(err).Msg("Prometheus metrics server failed")
			return
		}
	}()

	return nil
}
