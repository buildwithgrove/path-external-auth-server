// The portalappstore package contains the implementation of an in-memory store that stores
// PortalApps and their associated data from the Postgres database.
//
// It fetches this data from the database through an initial store update
// on startup, then listens for updates from the database to update the store.
package portalappstore

// dataSource defines the interface for a data source that provides portal apps.
//
// Satisfied by grove.GrovePostgresDriver
type DataSource interface {
	// FetchInitialData loads the initial set of portal apps.
	FetchInitialData() (map[PortalAppID]*PortalApp, error)

	// GetUpdateChannel returns a channel that provides updates to portal apps.
	GetUpdateChannel() <-chan PortalAppUpdate

	// Close closes the data source and cleans up any resources.
	Close()
}
