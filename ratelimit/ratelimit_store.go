package ratelimit

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pokt-network/poktroll/pkg/polylog"

	grovedb "github.com/buildwithgrove/path-external-auth-server/postgres/grove"
	"github.com/buildwithgrove/path-external-auth-server/store"
)

// TODO_IMPROVE(@commoddity): Get this from the database
const freeTierMonthlyUserLimit = 150_000

// accountRateLimitStore interface provides an in-memory store of account rate limits.
type accountRateLimitStore interface {
	GetAccountRateLimit(accountID store.AccountID) (store.RateLimit, bool)
}

// dataWarehouseDriver interface provides a driver for fetching monthly usage data from the data warehouse.
type dataWarehouseDriver interface {
	GetMonthToMomentUsage(ctx context.Context, minRelayThreshold int64) (map[string]int64, error)
}

// rateLimitStore provides an in-memory store of rate limited accounts.
type rateLimitStore struct {
	logger polylog.Logger

	dataWarehouseDriver   dataWarehouseDriver
	accountRateLimitStore accountRateLimitStore

	rateLimitedAccounts   map[store.AccountID]bool
	rateLimitedAccountsMu sync.RWMutex
}

func NewRateLimitStore(
	logger polylog.Logger,
	dataWarehouseDriver dataWarehouseDriver,
	accountRateLimitStore accountRateLimitStore,
	rateLimitUpdateInterval time.Duration,
) (*rateLimitStore, error) {
	rls := &rateLimitStore{
		logger: logger.With("component", "rate_limit_store"),

		accountRateLimitStore: accountRateLimitStore,
		dataWarehouseDriver:   dataWarehouseDriver,

		rateLimitedAccounts: make(map[store.AccountID]bool),
	}

	// Run initial check immediately
	if err := rls.updateRateLimitedAccounts(); err != nil {
		rls.logger.Error().
			Err(err).
			Msg("Failed to perform initial rate limit check")
	}

	// Start the background rate limit monitoring
	go rls.startRateLimitMonitoring(rateLimitUpdateInterval)

	return rls, nil
}

// IsAccountRateLimited checks if an account is currently rate limited.
func (rls *rateLimitStore) IsAccountRateLimited(accountID store.AccountID) bool {
	rls.rateLimitedAccountsMu.RLock()
	defer rls.rateLimitedAccountsMu.RUnlock()
	return rls.rateLimitedAccounts[accountID]
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
	usageData, err := rls.dataWarehouseDriver.GetMonthToMomentUsage(
		context.Background(),
		freeTierMonthlyUserLimit,
	)
	if err != nil {
		return fmt.Errorf("failed to get monthly usage data: %w", err)
	}

	// Build new rate limited accounts map
	newRateLimitedAccounts := make(map[store.AccountID]bool)

	for accountIDStr, usage := range usageData {
		accountID := store.AccountID(accountIDStr)

		// Get the account's rate limit configuration
		rateLimit, exists := rls.accountRateLimitStore.GetAccountRateLimit(accountID)
		if !exists {
			// Skip accounts without rate limit configuration
			continue
		}

		// Check if account should be rate limited based on plan type
		shouldLimit := rls.shouldLimitAccount(rateLimit, usage)
		if shouldLimit {
			newRateLimitedAccounts[accountID] = true
			rls.logger.Debug().
				Str("account_id", string(accountID)).
				Str("plan_type", string(rateLimit.PlanType)).
				Int64("usage", usage).
				Int32("monthly_limit", rateLimit.MonthlyUserLimit).
				Msg("🚫 Account rate limited")
		}
	}

	// Update the rate limited accounts map atomically
	rls.rateLimitedAccountsMu.Lock()
	rls.rateLimitedAccounts = newRateLimitedAccounts
	rls.rateLimitedAccountsMu.Unlock()

	updateDuration := time.Since(startTime)
	rls.logger.Info().
		Int("total_accounts_checked", len(usageData)).
		Int("rate_limited_accounts", len(newRateLimitedAccounts)).
		Int64("update_duration_ms", updateDuration.Milliseconds()).
		Msg("✅ Rate limit check completed")

	return nil
}

// shouldLimitAccount determines if an account should be rate limited based on its plan and usage.
func (rls *rateLimitStore) shouldLimitAccount(rateLimit store.RateLimit, usage int64) bool {
	switch rateLimit.PlanType {
	case grovedb.PlanFree_DatabaseType:
		// For free plan, check against the free tier limit
		return usage > freeTierMonthlyUserLimit

	case grovedb.PlanUnlimited_DatabaseType:
		// For unlimited plan, check against the account's specific monthly limit (if set)
		if rateLimit.MonthlyUserLimit > 0 {
			return usage > int64(rateLimit.MonthlyUserLimit)
		}
		// If no limit is set for unlimited plan, don't rate limit
		return false

	default:
		// Unknown plan type - don't rate limit by default
		rls.logger.Warn().
			Str("plan_type", string(rateLimit.PlanType)).
			Msg("Unknown plan type encountered")
		return false
	}
}
