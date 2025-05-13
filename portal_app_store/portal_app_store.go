// The portalappstore package contains the implementation of an in-memory store that stores
// PortalApps and their associated data from the Postgres database.
//
// It fetches this data from the database through an initial store update
// on startup, then listens for updates from the database to update the store.
package portalappstore

import (
	"context"
	"fmt"
	"sync"

	"github.com/pokt-network/poktroll/pkg/polylog"
)

// portalAppStore is an in-memory store that stores portal apps and their associated data.
type portalAppStore struct {
	logger polylog.Logger

	// The data source for portal apps
	dataSource DataSource

	portalApps   map[PortalAppID]*PortalApp
	portalAppsMu sync.RWMutex
}

// NewPortalAppStore creates a new portal app store, which stores PortalApps in memory for fast access.
// It initializes the store by requesting data from the data source and listens for updates from the update channel.
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

	err := store.initializeStore()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize portal app store: %w", err)
	}

	// Start listening for updates from the database.
	go store.listenForUpdates(context.Background())

	return store, nil
}

// GetPortalApp returns a PortalApp from the store and a bool indicating if it exists in the store.
func (c *portalAppStore) GetPortalApp(portalAppID PortalAppID) (*PortalApp, bool) {
	c.portalAppsMu.RLock()
	defer c.portalAppsMu.RUnlock()

	gatewayPortalApp, ok := c.portalApps[portalAppID]
	return gatewayPortalApp, ok
}

// initializeStore requests the initial data from the data source to populate the store.
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

// listenForUpdates listens for updates from the update channel and updates the store accordingly.
// Updates will be one of three cases:
//  1. A new PortalApp was created
//  2. An existing PortalApp was updated
//  3. An existing PortalApp was deleted
func (c *portalAppStore) listenForUpdates(ctx context.Context) {
	updatesCh := c.dataSource.GetUpdateChannel()

	for {
		select {
		case <-ctx.Done():
			c.logger.Info().Msg("context cancelled, stopping update listener")
			return

		case update := <-updatesCh:
			c.portalAppsMu.Lock()
			if update.Delete {
				delete(c.portalApps, update.PortalAppID)
				c.logger.Info().Str("portal_app_id", string(update.PortalAppID)).Msg("deleted portal app")
			} else {
				c.portalApps[update.PortalAppID] = update.PortalApp
				c.logger.Info().Str("portal_app_id", string(update.PortalAppID)).Msg("updated portal app")
			}
			c.portalAppsMu.Unlock()
		}
	}
}
