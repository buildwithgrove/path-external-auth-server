package store

import (
	"fmt"
	"sync"
	"time"

	"github.com/pokt-network/poktroll/pkg/polylog"
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

	// Mutex to protect access to portalApps
	portalAppsMu sync.RWMutex
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
		logger:       logger.With("component", "portal_app_data_store"),
		dataSource:   dataSource,
		portalApps:   make(map[PortalAppID]*PortalApp),
		portalAppsMu: sync.RWMutex{},
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

// initializeStore fetches the initial set of PortalApps from the data source and populates the in-memory store.
func (c *portalAppStore) initializeStore() error {
	c.logger.Info().Msg("Fetching initial data from data source ...")

	portalApps, err := c.dataSource.GetPortalApps()
	if err != nil {
		return fmt.Errorf("failed to get initial data from data source: %w", err)
	}

	c.logger.Info().Msg("🌱 Successfully fetched initial data from data source")

	c.portalAppsMu.Lock()
	defer c.portalAppsMu.Unlock()
	c.portalApps = portalApps

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

	portalApps, err := c.dataSource.GetPortalApps()
	if err != nil {
		return fmt.Errorf("failed to get portal apps from data source: %w", err)
	}

	c.portalAppsMu.Lock()
	defer c.portalAppsMu.Unlock()
	c.portalApps = portalApps

	refreshDuration := time.Since(startTime)
	c.logger.Debug().
		Int("portal_app_count", len(portalApps)).
		Int64("refresh_duration_ms", refreshDuration.Milliseconds()).
		Msg("🌿 Successfully refreshed portal apps from data source")

	return nil
}
