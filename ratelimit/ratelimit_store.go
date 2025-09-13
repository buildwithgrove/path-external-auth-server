package ratelimit

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pokt-network/poktroll/pkg/polylog"

	"github.com/buildwithgrove/path-external-auth-server/metrics"
	grovedb "github.com/buildwithgrove/path-external-auth-server/postgres/grove"
	"github.com/buildwithgrove/path-external-auth-server/store"
)

// 💡IMPORTANT💡: This value is used to determine the PLAN_FREE monthly relay limit.
//
// TODO_TECHDEBT(@commoddity): When the Portal DB is implemented,
// this value should be fetched from the Postgres database.
//
// Once PLAN_FREE accounts hit this limit, they are rate limited until the start of the next month.
const FreeMonthlyRelays = 1_000_000

// accountPortalAppStore interface provides an in-memory store of account portal apps.
type accountPortalAppStore interface {
	GetAccountPortalApp(accountID store.AccountID) (*store.PortalApp, bool)
}

// dataWarehouseDriver interface provides a driver for fetching monthly usage data from the data warehouse.
type dataWarehouseDriver interface {
	GetMonthToMomentUsage(ctx context.Context, minRelayThreshold int64) (map[string]int64, error)
}

// rateLimitStore provides an in-memory store of rate limited accounts.
type rateLimitStore struct {
	logger polylog.Logger

	dataWarehouseDriver   dataWarehouseDriver
	accountPortalAppStore accountPortalAppStore

	rateLimitedAccounts   map[store.AccountID]bool
	rateLimitedAccountsMu sync.RWMutex
}

func NewRateLimitStore(
	logger polylog.Logger,
	dataWarehouseDriver dataWarehouseDriver,
	accountPortalAppStore accountPortalAppStore,
	rateLimitUpdateInterval time.Duration,
) (*rateLimitStore, error) {
	rls := &rateLimitStore{
		logger: logger.With("component", "rate_limit_store"),

		accountPortalAppStore: accountPortalAppStore,
		dataWarehouseDriver:   dataWarehouseDriver,

		rateLimitedAccounts: make(map[store.AccountID]bool),
	}

	// Run initial check immediately
	if err := rls.updateRateLimitedAccounts(); err != nil {
		rls.logger.Error().
			Err(err).
			Msg("Failed to perform initial rate limit check")
		// Set initial metrics to zero if initial check fails
		rls.updateStoreMetrics(0, 0)
	}

	// Start the background rate limit monitoring
	go rls.startRateLimitMonitoring(rateLimitUpdateInterval)

	return rls, nil
}

// IsAccountRateLimited checks if an account is currently rate limited.
func (rls *rateLimitStore) IsAccountRateLimited(accountID store.AccountID) bool {
	rls.rateLimitedAccountsMu.RLock()
	defer rls.rateLimitedAccountsMu.RUnlock()
	_, ok := rls.rateLimitedAccounts[accountID]
	return ok
}

// startRateLimitMonitoring runs the periodic rate limit check in a background goroutine.
func (rls *rateLimitStore) startRateLimitMonitoring(rateLimitUpdateInterval time.Duration) {
	rls.logger.Info().
		Dur("update_interval", rateLimitUpdateInterval).
		Msg("🚦 Starting rate limit monitoring")

	ticker := time.NewTicker(rateLimitUpdateInterval)
	defer ticker.Stop()

	for range ticker.C {
		if err := rls.updateRateLimitedAccounts(); err != nil {
			rls.logger.Error().
				Err(err).
				Msg("Failed to update rate limited accounts")
		}
	}
}

// updateRateLimitedAccounts fetches usage data and updates the rate limited accounts map.
func (rls *rateLimitStore) updateRateLimitedAccounts() error {
	startTime := time.Now()
	rls.logger.Debug().Msg("🔍 Checking account rate limits")

	// Get month-to-date usage for accounts over the threshold
	accountUsageOverMonthlyRelayLimit, err := rls.dataWarehouseDriver.GetMonthToMomentUsage(
		context.Background(),
		FreeMonthlyRelays,
	)
	if err != nil {
		metrics.RecordDataSourceRefreshError("rate_limit_store", "bigquery_error")
		return fmt.Errorf("failed to get monthly usage data: %w", err)
	}

	// Build new rate limited accounts map
	newRateLimitedAccounts := make(map[store.AccountID]bool)

	for accountIDStr, usage := range accountUsageOverMonthlyRelayLimit {
		accountID := store.AccountID(accountIDStr)

		// Get the account's portal app
		portalApp, exists := rls.accountPortalAppStore.GetAccountPortalApp(accountID)
		if !exists {
			// Skip accounts without rate limit configuration
			continue
		}

		// Get the account's rate limit
		// Will return 0 if no rate limit is configured
		rateLimit := rls.getRateLimit(portalApp)

		// Update account usage metrics for accounts over monthly limit
		planType := string(portalApp.PlanType)
		metrics.UpdateAccountUsage(string(accountID), planType, float64(usage), rateLimit)

		// Check if account should be rate limited based on plan type
		shouldLimit := rls.shouldLimitAccount(rateLimit, usage)
		if shouldLimit {
			newRateLimitedAccounts[accountID] = true
			metrics.UpdateRateLimitedAccounts(string(accountID), planType, float64(usage), rateLimit)
			rls.logger.Debug().
				Str("account_id", string(accountID)).
				Str("plan_type", string(portalApp.PlanType)).
				Int64("usage", usage).
				Msg("🚫 Account rate limited")
		}
	}

	// Update the rate limited accounts map atomically
	rls.rateLimitedAccountsMu.Lock()
	rls.rateLimitedAccounts = newRateLimitedAccounts
	rls.rateLimitedAccountsMu.Unlock()

	// Update store size metrics
	rls.updateStoreMetrics(len(accountUsageOverMonthlyRelayLimit), len(newRateLimitedAccounts))

	updateDuration := time.Since(startTime)
	rls.logger.Info().
		Int("total_accounts_over_monthly_relay_limit", len(accountUsageOverMonthlyRelayLimit)).
		Int("rate_limited_accounts", len(newRateLimitedAccounts)).
		Int64("update_duration_ms", updateDuration.Milliseconds()).
		Msg("✅ Rate limit check completed")

	return nil
}

// getRateLimit gets the rate limit for an account based on its plan type and rate limit configuration.
func (rls *rateLimitStore) getRateLimit(portalApp *store.PortalApp) int32 {
	if portalApp.RateLimit == nil {
		return 0
	}

	switch portalApp.PlanType {
	case grovedb.PlanFree_DatabaseType:
		// For free plan, return the free tier limit
		return FreeMonthlyRelays
	case grovedb.PlanUnlimited_DatabaseType:
		// For unlimited plan, check against the account's specific monthly limit (if set)
		if portalApp.RateLimit.MonthlyUserLimit > 0 {
			return portalApp.RateLimit.MonthlyUserLimit
		}
		// If no limit is set for unlimited plan, don't rate limit
		return 0
	default:
		return 0
	}
}

// shouldLimitAccount determines if an account should be rate limited based on its rate limit and usage.
func (rls *rateLimitStore) shouldLimitAccount(rateLimit int32, usage int64) bool {
	// If rate limit is 0, don't rate limit (unlimited)
	if rateLimit == 0 {
		return false
	}
	return usage > int64(rateLimit)
}

// updateStoreMetrics updates the Prometheus metrics for rate limit store sizes.
func (rls *rateLimitStore) updateStoreMetrics(accountsOverLimit, rateLimitedAccounts int) {
	metrics.UpdateStoreSize("accounts_over_monthly_limit", float64(accountsOverLimit))
	metrics.UpdateStoreSize("rate_limited_accounts", float64(rateLimitedAccounts))
}
