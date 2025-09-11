package store

import (
	"fmt"
	"sync"
	"time"

	"github.com/pokt-network/poktroll/pkg/polylog"

	"github.com/buildwithgrove/path-external-auth-server/metrics"
)

// portalAppStore is an in-memory store for portal apps and their associated data.
//
// Responsibilities:
// - Maintain a fast-access, thread-safe map of PortalApps
// - Sync with a data source for initial and live updates
// - Provide lookup methods for PortalApps
type portalAppStore struct {
	logger polylog.Logger

	// Data source for fetching and updating portal apps
	dataSource DataSource

	// In-memory map of portal apps (portalAppID -> *PortalApp)
	portalApps map[PortalAppID]*PortalApp

	// In-memory map of account rate limits (accountID -> RateLimit)
	accountRateLimits map[AccountID]RateLimit

	// Mutex to protect access to portalApps
	portalAppsMu sync.RWMutex

	// Mutex to protect access to accountRateLimits
	accountRateLimitsMu sync.RWMutex
}

// NewPortalAppStore creates a new in-memory portal app store.
//
// Steps:
// - Initializes the store with initial data from the data source
// - Starts a goroutine to listen for live updates from the data source
// - Returns the initialized store or error if initialization fails
func NewPortalAppStore(
	logger polylog.Logger,
	dataSource DataSource,
	refreshInterval time.Duration,
) (*portalAppStore, error) {
	store := &portalAppStore{
		logger:            logger.With("component", "portal_app_data_store"),
		dataSource:        dataSource,
		portalApps:        make(map[PortalAppID]*PortalApp),
		accountRateLimits: make(map[AccountID]RateLimit),
	}

	// Fetch initial data from the data source and populate the store
	err := store.initializeStore()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize portal app store: %w", err)
	}

	// Start background refresh goroutine
	go store.startBackgroundRefresh(refreshInterval)

	return store, nil
}

// GetPortalApp retrieves a PortalApp from the store by its ID.
//
// Returns:
// - The PortalApp pointer if found
// - A bool indicating if the PortalApp exists in the store
func (c *portalAppStore) GetPortalApp(portalAppID PortalAppID) (*PortalApp, bool) {
	c.portalAppsMu.RLock()
	defer c.portalAppsMu.RUnlock()

	portalApp, ok := c.portalApps[portalAppID]
	return portalApp, ok
}

// GetAccountRateLimit retrieves a RateLimit from the store by its account ID.
//
// Returns:
// - The RateLimit pointer if found
// - A bool indicating if the RateLimit exists in the store
func (c *portalAppStore) GetAccountRateLimit(accountID AccountID) (RateLimit, bool) {
	c.accountRateLimitsMu.RLock()
	defer c.accountRateLimitsMu.RUnlock()

	rateLimit, ok := c.accountRateLimits[accountID]
	return rateLimit, ok
}

// initializeStore fetches the initial set of PortalApps from the data source and populates the in-memory store.
func (c *portalAppStore) initializeStore() error {
	c.logger.Info().Msg("Fetching initial data from data source ...")

	err := c.setStoreData()
	if err != nil {
		metrics.RecordDataSourceRefreshError("portal_app_store", "postgres_error")
		return fmt.Errorf("failed to set initial store data: %w", err)
	}

	// Update initial store size metrics
	c.updateStoreMetrics()

	c.logger.Info().Msg("🌱 Successfully fetched initial data from data source")
	return nil
}

// startBackgroundRefresh starts a goroutine that periodically refreshes the portal apps from the data source.
func (c *portalAppStore) startBackgroundRefresh(refreshInterval time.Duration) {
	c.logger.Info().
		Dur("refresh_interval", refreshInterval).
		Msg("🗄️ Starting background refresh for portal apps")

	ticker := time.NewTicker(refreshInterval)
	defer ticker.Stop()

	for range ticker.C {
		if err := c.refreshStore(); err != nil {
			c.logger.Error().
				Err(err).
				Msg("Failed to refresh portal apps from data source")
		}
	}
}

// refreshStore fetches the latest PortalApps from the data source and updates the in-memory store.
func (c *portalAppStore) refreshStore() error {
	startTime := time.Now()
	c.logger.Debug().Msg("💡 Refreshing portal apps from data source")

	err := c.setStoreData()
	if err != nil {
		metrics.RecordDataSourceRefreshError("portal_app_store", "postgres_error")
		return fmt.Errorf("failed to refresh store data: %w", err)
	}

	refreshDuration := time.Since(startTime)
	c.logger.Debug().
		Int("portal_app_count", len(c.portalApps)).
		Int64("refresh_duration_ms", refreshDuration.Milliseconds()).
		Msg("🌿 Successfully refreshed portal apps from data source")

	// Update store size metrics
	c.updateStoreMetrics()

	return nil
}

// setStoreData fetches portal apps from the data source and updates both portal apps and account rate limits.
// This method is used by both initializeStore and refreshStore to avoid code duplication.
func (c *portalAppStore) setStoreData() error {
	portalApps, err := c.dataSource.GetPortalApps()
	if err != nil {
		return fmt.Errorf("failed to get portal apps from data source: %w", err)
	}

	c.portalAppsMu.Lock()
	c.portalApps = portalApps
	c.portalAppsMu.Unlock()

	c.setAccountRateLimits(portalApps)

	return nil
}

// updateStoreMetrics updates the Prometheus metrics for store sizes.
func (c *portalAppStore) updateStoreMetrics() {
	c.portalAppsMu.RLock()
	portalAppCount := len(c.portalApps)
	c.portalAppsMu.RUnlock()

	c.accountRateLimitsMu.RLock()
	accountCount := len(c.accountRateLimits)
	c.accountRateLimitsMu.RUnlock()

	// Update store size metrics
	metrics.UpdateStoreSize("portal_apps", float64(portalAppCount))
	metrics.UpdateStoreSize("accounts", float64(accountCount))
}

// setAccountRateLimits extracts and caches account rate limits from portal apps.
// Only sets account data if it's not already set for the same account ID to avoid unnecessary updates.
func (c *portalAppStore) setAccountRateLimits(portalApps map[PortalAppID]*PortalApp) {
	c.accountRateLimitsMu.Lock()
	defer c.accountRateLimitsMu.Unlock()

	for _, portalApp := range portalApps {
		// Skip if portal app has no rate limit configured
		if portalApp.RateLimit == nil {
			continue
		}

		// Only set if not already present for this account ID
		if _, exists := c.accountRateLimits[portalApp.AccountID]; !exists {
			c.accountRateLimits[portalApp.AccountID] = *portalApp.RateLimit
		}
	}
}
