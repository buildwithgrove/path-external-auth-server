package metrics

import (
	"fmt"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
)

// TODO_TESTING: Add comprehensive unit tests for metrics recording functions
// TODO_PERFORMANCE: Consider implementing metric batching for high-volume scenarios
// TODO_MONITORING: Add metric validation to ensure label values are within expected ranges

// See the metrics initialization below for details.
const (
	// The POSIX process that emits metrics
	peasProcess = "peas"

	// Authorization request metrics
	authRequestsTotalMetricName          = "auth_requests_total"
	authRequestDurationSecondsMetricName = "auth_request_duration_seconds"

	// Rate limiting metrics
	rateLimitChecksTotalMetricName = "rate_limit_checks_total"

	// Store size metrics
	storeSizeTotalMetricName = "store_size_total"

	// Account usage tracking
	accountUsageTotalMetricName = "account_usage_total"

	// Data source refresh error tracking
	dataSourceRefreshErrorsTotalMetricName = "data_source_refresh_errors_total"
)

func init() {
	prometheus.MustRegister(authRequestsTotal)
	prometheus.MustRegister(authRequestDurationSeconds)
	prometheus.MustRegister(rateLimitChecksTotal)
	prometheus.MustRegister(storeSizeTotal)
	prometheus.MustRegister(accountUsageTotal)
	prometheus.MustRegister(rateLimitedAccountsTotal)
	prometheus.MustRegister(dataSourceRefreshErrorsTotal)
}

var (
	// authRequestsTotal tracks all authorization requests processed by PEAS.
	// Increment on each Check request with labels:
	//   - portal_app_id: Portal app making the request
	//   - account_id: Account associated with the portal app
	//   - status: "authorized", "denied", "error"
	//   - error_type: "portal_app_not_found", "unauthorized", "rate_limited", "invalid_request", "internal_error", or empty for success
	//
	// Usage:
	// - Monitor total authorization load per portal app and account
	// - Track success/failure rates by error type
	// - Analyze portal app ID extraction patterns
	authRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: peasProcess,
			Name:      authRequestsTotalMetricName,
			Help:      "Total authorization requests processed, labeled by portal app, account, status and error details.",
		},
		[]string{"portal_app_id", "account_id", "status", "error_type"},
	)

	// authRequestDurationSeconds measures authorization request processing duration.
	// Histogram buckets from 100ns to 10ms capture performance from fast in-memory lookups to slower operations.
	//
	// Usage:
	// - Monitor authorization latency SLAs
	// - Identify performance issues per portal app
	// - Track impact of rate limiting checks on response time
	authRequestDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Subsystem: peasProcess,
			Name:      authRequestDurationSecondsMetricName,
			Help:      "Histogram of authorization request processing time in seconds",
			// Buckets optimized for very fast in-memory operations (100ns to 10ms)
			Buckets: []float64{
				0.0000001, 0.0000005, 0.000001, 0.000005, 0.00001,
				0.00005, 0.0001, 0.0005, 0.001, 0.005, 0.01,
			},
		},
		[]string{"portal_app_id", "status"},
	)

	// rateLimitChecksTotal tracks rate limiting decisions made by PEAS.
	// Increment for each rate limit check with labels:
	//   - account_id: Account being checked
	//   - plan_type: "PLAN_FREE", "PLAN_UNLIMITED"
	//   - decision: "allowed", "rate_limited", "no_limit_configured"
	//
	// Usage:
	// - Monitor rate limiting effectiveness by plan type
	// - Track which accounts are being rate limited
	// - Analyze rate limiting patterns
	rateLimitChecksTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: peasProcess,
			Name:      rateLimitChecksTotalMetricName,
			Help:      "Total rate limit checks performed, labeled by account, plan type and decision.",
		},
		[]string{"account_id", "plan_type", "decision"},
	)

	// storeSizeTotal tracks the current size of in-memory stores.
	// Set as gauge with labels:
	//   - store_type: "accounts", "portal_apps", "rate_limited_accounts", "accounts_over_monthly_limit"
	//
	// Usage:
	// - Monitor store growth over time
	// - Capacity planning for memory usage
	// - Track rate limiting effectiveness
	storeSizeTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: peasProcess,
			Name:      storeSizeTotalMetricName,
			Help:      "Current size of in-memory stores by type.",
		},
		[]string{"store_type"},
	)

	// accountUsageTotal tracks monthly usage for accounts that exceed their monthly limit.
	// Set as gauge with labels:
	//   - account_id: Account ID that is over the monthly limit
	//   - plan_type: "PLAN_FREE", "PLAN_UNLIMITED"
	//   - rate_limit: monthly rate limit for the account
	//
	// Note: Usage values only increase during the month and reset at month boundaries.
	// Use Grafana queries with time-based filtering to show current month data only.
	//
	// Usage:
	// - Monitor which specific accounts are over their limits
	// - Track usage patterns for rate limited accounts
	// - Identify high-usage accounts for potential plan upgrades
	accountUsageTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: peasProcess,
			Name:      accountUsageTotalMetricName,
			Help:      "Monthly usage for accounts that exceed their monthly limit.",
		},
		[]string{"account_id", "plan_type", "rate_limit"},
	)

	// rateLimitedAccountsTotal tracks accounts that are currently rate limited.
	// Set as gauge with labels:
	//   - account_id: Account ID that is rate limited
	//   - plan_type: "PLAN_FREE", "PLAN_UNLIMITED"
	//   - monthly_usage: Current monthly usage
	//   - rate_limit: monthly rate limit for the account
	//
	// Usage:
	// - Monitor which specific accounts are currently rate limited
	// - Track usage patterns for rate limited accounts
	rateLimitedAccountsTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: peasProcess,
			Name:      "rate_limited_accounts_total",
			Help:      "Accounts currently rate limited, labeled by account ID, plan type, and monthly usage.",
		},
		[]string{"account_id", "plan_type", "monthly_usage", "rate_limit"},
	)

	// dataSourceRefreshErrorsTotal tracks errors during data source refresh operations.
	// Increment on refresh errors with labels:
	//   - source_type: "portal_app_store", "rate_limit_store"
	//   - error_type: "postgres_error", "bigquery_error", "connection_error", "timeout_error"
	//
	// Usage:
	// - Monitor data source health and reliability
	// - Alert on refresh failures that could impact authorization
	// - Track error patterns by source type
	dataSourceRefreshErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: peasProcess,
			Name:      dataSourceRefreshErrorsTotalMetricName,
			Help:      "Total errors during data source refresh operations.",
		},
		[]string{"source_type", "error_type"},
	)
)

// RecordAuthRequest records an authorization request with all relevant labels.
func RecordAuthRequest(
	portalAppID string,
	accountID string,
	status string,
	errorType string,
	duration float64,
) {
	authRequestsTotal.With(prometheus.Labels{
		"portal_app_id": portalAppID,
		"account_id":    accountID,
		"status":        status,
		"error_type":    errorType,
	}).Inc()

	authRequestDurationSeconds.With(prometheus.Labels{
		"portal_app_id": portalAppID,
		"status":        status,
	}).Observe(duration)
}

// RecordRateLimitCheck records a rate limit check decision.
func RecordRateLimitCheck(
	accountID string,
	planType string,
	decision string,
) {
	rateLimitChecksTotal.With(prometheus.Labels{
		"account_id": accountID,
		"plan_type":  planType,
		"decision":   decision,
	}).Inc()
}

// UpdateStoreSize updates the current size of a store.
func UpdateStoreSize(
	storeType string,
	size float64,
) {
	storeSizeTotal.With(prometheus.Labels{
		"store_type": storeType,
	}).Set(size)
}

// UpdateAccountUsage updates the usage for an account that is over their monthly limit.
func UpdateAccountUsage(
	accountID string,
	planType string,
	monthlyUsage float64,
	rateLimit int32,
) {
	accountUsageTotal.With(prometheus.Labels{
		"account_id": accountID,
		"plan_type":  planType,
		"rate_limit": strconv.FormatInt(int64(rateLimit), 10),
	}).Set(monthlyUsage)
}

// UpdateRateLimitedAccounts updates the rate limited accounts metric with current usage data.
func UpdateRateLimitedAccounts(
	accountID string,
	planType string,
	monthlyUsage float64,
	rateLimit int32,
) {
	rateLimitedAccountsTotal.With(prometheus.Labels{
		"account_id":    accountID,
		"plan_type":     planType,
		"monthly_usage": fmt.Sprintf("%.2f", monthlyUsage),
		"rate_limit":    strconv.FormatInt(int64(rateLimit), 10),
	}).Set(monthlyUsage)
}

// RecordDataSourceRefreshError records an error during data source refresh.
func RecordDataSourceRefreshError(
	sourceType string,
	errorType string,
) {
	dataSourceRefreshErrorsTotal.With(prometheus.Labels{
		"source_type": sourceType,
		"error_type":  errorType,
	}).Inc()
}
