package store

// DataSource defines the interface for a data source that provides portal apps.
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
