package store

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pokt-network/poktroll/pkg/polylog"
)

// portalAppStore is an in-memory store for portal apps and their associated data.
//
// Responsibilities:
// - Maintain a fast-access, thread-safe map of PortalApps
// - Sync with a data source for initial and periodic updates
// - Provide lookup methods for PortalApps
type portalAppStore struct {
	logger polylog.Logger

	// Data source for fetching and updating portal apps
	dataSource DataSource

	// In-memory map of portal apps (portalAppID -> *PortalApp)
	portalApps map[PortalAppID]*PortalApp

	// Mutex to protect access to portalApps
	portalAppsMu sync.RWMutex

	// Ticker for periodic updates
	ticker *time.Ticker

	// Context for controlling the refresh goroutine
	ctx    context.Context
	cancel context.CancelFunc
}

// NewPortalAppStore creates a new in-memory portal app store.
//
// Steps:
// - Initializes the store with initial data from the data source
// - Starts a goroutine to periodically refresh data every 30 seconds
// - Returns the initialized store or error if initialization fails
func NewPortalAppStore(
	logger polylog.Logger,
	dataSource DataSource,
) (*portalAppStore, error) {
	ctx, cancel := context.WithCancel(context.Background())

	store := &portalAppStore{
		logger:       logger.With("component", "portal_app_data_store"),
		dataSource:   dataSource,
		portalApps:   make(map[PortalAppID]*PortalApp),
		portalAppsMu: sync.RWMutex{},
		ticker:       time.NewTicker(30 * time.Second),
		ctx:          ctx,
		cancel:       cancel,
	}

	// Fetch initial data from the data source and populate the store
	err := store.initializeStore()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to initialize portal app store: %w", err)
	}

	// Start a background goroutine to periodically refresh data
	go store.periodicRefresh()

	return store, nil
}

// GetPortalApp retrieves a PortalApp from the store by its ID.
//
// TODO_TECHDEBT(@adshmh): Review the use of pointers from cached data - with periodic cache
// refresh replacing the entire map every 30 seconds, returning pointers to cached objects
// could potentially lead to stale references if callers hold onto them for extended periods.
// Consider returning copies instead of pointers for better safety.
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

// Stop gracefully shuts down the portal app store by stopping the periodic refresh.
func (c *portalAppStore) Stop() {
	c.logger.Info().Msg("stopping portal app store")
	c.ticker.Stop()
	c.cancel()
}

// initializeStore fetches the initial set of PortalApps from the data source and populates the in-memory store.
func (c *portalAppStore) initializeStore() error {
	c.logger.Info().Msg("fetching initial data from data source")

	portalApps, err := c.dataSource.FetchInitialData()
	if err != nil {
		return fmt.Errorf("failed to get initial data from data source: %w", err)
	}

	c.portalAppsMu.Lock()
	defer c.portalAppsMu.Unlock()
	c.portalApps = portalApps

	c.logger.Info().
		Int("portal_apps_count", len(portalApps)).
		Msg("successfully initialized portal app store")

	return nil
}

// periodicRefresh continuously refreshes portal app data from the data source every 30 seconds.
//
// This replaces the previous listener-based approach with a simple periodic fetch that:
// - Fetches all current data from the data source
// - Replaces the entire in-memory cache
// - Logs refresh status and any errors
//
// Exits when the context is cancelled.
func (c *portalAppStore) periodicRefresh() {
	c.logger.Info().Msg("starting periodic refresh every 30 seconds")

	for {
		select {
		case <-c.ctx.Done():
			c.logger.Info().Msg("context cancelled, stopping periodic refresh")
			return

		case <-c.ticker.C:
			c.logger.Debug().Msg("refreshing portal apps from data source")

			// Fetch fresh data from the data source
			portalApps, err := c.dataSource.FetchInitialData()
			if err != nil {
				c.logger.Error().
					Err(err).
					Msg("failed to refresh data from data source")
				continue
			}

			// Update the in-memory store with fresh data
			c.portalAppsMu.Lock()
			oldCount := len(c.portalApps)
			c.portalApps = portalApps
			newCount := len(portalApps)
			c.portalAppsMu.Unlock()

			c.logger.Info().
				Int("previous_count", oldCount).
				Int("current_count", newCount).
				Msg("successfully refreshed portal apps")
		}
	}
}
