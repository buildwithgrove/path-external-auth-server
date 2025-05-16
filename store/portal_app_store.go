package store

import (
	"context"
	"fmt"
	"sync"

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

	// Start a background goroutine to listen for live updates
	go store.listenForUpdates(context.Background())

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

	portalApps, err := c.dataSource.FetchInitialData()
	if err != nil {
		return fmt.Errorf("failed to get initial data from data source: %w", err)
	}

	c.logger.Info().Msg("Successfully fetched initial data from data source")

	c.portalAppsMu.Lock()
	defer c.portalAppsMu.Unlock()
	c.portalApps = portalApps

	return nil
}

// listenForUpdates continuously listens for portal app updates from the data source and applies them to the store.
//
// Update cases handled:
// - New PortalApp created
// - Existing PortalApp updated
// - Existing PortalApp deleted
//
// Exits when the context is cancelled.
func (c *portalAppStore) listenForUpdates(ctx context.Context) {
	updatesCh := c.dataSource.GetUpdateChannel()

	for {
		select {
		case <-ctx.Done():
			// Stop listening for updates if the context is cancelled
			c.logger.Info().Msg("context cancelled, stopping update listener")
			return

		case update := <-updatesCh:
			c.portalAppsMu.Lock()
			if update.Delete {
				// Remove PortalApp from store if marked for deletion
				delete(c.portalApps, update.PortalAppID)
				c.logger.Info().Str("portal_app_id", string(update.PortalAppID)).Msg("deleted portal app")
			} else {
				// Add or update PortalApp in store
				c.portalApps[update.PortalAppID] = update.PortalApp
				c.logger.Info().Str("portal_app_id", string(update.PortalAppID)).Msg("updated portal app")
			}
			c.portalAppsMu.Unlock()
		}
	}
}
