// The store package contains the implementation of an in-memory store that stores
// PortalApps and their associated data from the Postgres database.
//
// It fetches this data from the database through an initial store update
// on startup, then listens for updates from the database to update the store.
package store
