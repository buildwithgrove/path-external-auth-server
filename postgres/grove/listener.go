package grove

import (
	"context"
	"fmt"
	"time"

	store "github.com/buildwithgrove/path-external-auth-server/portal_app_store"
	"github.com/buildwithgrove/path-external-auth-server/postgres/grove/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgxlisten"
	"github.com/pokt-network/poktroll/pkg/polylog"
)

/* ---------- Data Update Listener Funcs ---------- */

const portalApplicationChangesChannel = "portal_application_changes"

type Notification struct {
	Payload string
}

type PGXNotificationHandler struct {
	outCh chan *Notification
}

func (h *PGXNotificationHandler) HandleNotification(ctx context.Context, n *pgconn.Notification, conn *pgx.Conn) error {
	h.outCh <- &Notification{Payload: n.Payload}
	return nil
}

// newPGXPoolListener creates a new pgxlisten.Listener with a connection from the provided pool and output channel.
// It listens for updates from the Postgres database, using the function and triggers defined in "./postgres/sqlc/grove_triggers.sql"
func newPGXPoolListener(pool *pgxpool.Pool, logger polylog.Logger) *pgxlisten.Listener {
	connectFunc := func(ctx context.Context) (*pgx.Conn, error) {
		// Set a timeout for acquiring a connection
		acquireCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		conn, err := pool.Acquire(acquireCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to acquire connection: %v", err)
		}
		return conn.Conn(), nil
	}

	listener := &pgxlisten.Listener{
		Connect: connectFunc,
		LogError: func(ctx context.Context, err error) {
			logger.Error().Err(err).Msg("listener error")
		},
	}

	return listener
}

func (d *GrovePostgresDriver) listenForUpdates(ctx context.Context) {
	handler := &PGXNotificationHandler{outCh: d.notificationCh}
	d.listener.Handle(portalApplicationChangesChannel, handler)

	go func() {
		if err := d.listener.Listen(ctx); err != nil {
			d.logger.Error().Err(err).Msg("error initializing listener")
		}
	}()

	go func() {
		for {
			select {
			case <-ctx.Done():
				d.logger.Info().Msg("context cancelled, stopping notification processor")
				return

			case notification := <-d.notificationCh:
				// Process the notification
				if notification == nil {
					continue
				}

				// Create a context with timeout for processing changes
				processCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				err := d.processPortalAppChanges(processCtx)
				cancel()
				if err != nil {
					d.logger.Error().Err(err).Msg("failed to process portal application changes")
				}
			}
		}
	}()
}

// processPortalAppChanges is the main method that processes changes from the portal_application_changes table.
// It coordinates the entire lifecycle of change processing in a single transaction:
//  1. Start a transaction
//  2. Clean up old processed changes
//  3. Get unprocessed changes
//  4. Process each change (sending updates to the channel)
//  5. Mark changes as processed
//  6. Commit the transaction
func (d *GrovePostgresDriver) processPortalAppChanges(ctx context.Context) error {
	// Start a transaction for the entire process
	tx, err := d.driver.DB.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Track if transaction was committed or needs to be rolled back
	var committed bool
	defer func() {
		// Only rollback if not committed
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	// Create a Queries instance that uses our transaction
	qtx := d.driver.Queries.WithTx(tx)

	// Clean up old processed changes
	if err = d.cleanupProcessedChanges(ctx, qtx); err != nil {
		return err
	}

	// Get and process unprocessed changes
	changeIDs, err := d.fetchAndProcessChanges(ctx, qtx)
	if err != nil {
		return err
	}

	// If there were no changes, just commit and return
	if len(changeIDs) == 0 {
		if err = tx.Commit(ctx); err != nil {
			return fmt.Errorf("failed to commit transaction: %w", err)
		}
		committed = true
		return nil
	}

	// Mark changes as processed
	if err = d.markChangesAsProcessed(ctx, qtx, changeIDs); err != nil {
		return err
	}

	// Commit the transaction
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	committed = true

	return nil
}

// cleanupProcessedChanges removes old processed changes from the database.
// This ensures the changes table doesn't grow indefinitely.
func (d *GrovePostgresDriver) cleanupProcessedChanges(ctx context.Context, qtx *sqlc.Queries) error {
	if err := qtx.DeleteProcessedPortalAppChanges(ctx); err != nil {
		return fmt.Errorf("failed to clean up processed changes: %w", err)
	}
	return nil
}

// fetchAndProcessChanges retrieves unprocessed changes from the database and processes them,
// sending appropriate updates to the updatesCh channel.
// Returns the IDs of processed changes and any error encountered.
func (d *GrovePostgresDriver) fetchAndProcessChanges(ctx context.Context, qtx *sqlc.Queries) ([]int32, error) {
	// Get unprocessed changes
	changes, err := qtx.GetPortalAppChanges(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get portal application changes: %w", err)
	}

	if len(changes) == 0 {
		return nil, nil // No changes to process
	}

	var changeIDs []int32

	// Process each change
	for _, change := range changes {
		// Process the change and collect its ID
		changeIDs = append(changeIDs, change.ID)

		// Handle the change based on its type
		if change.IsDelete {
			d.handleDeleteChange(change)
		} else {
			d.handleUpsertChange(ctx, qtx, change)
		}
	}

	return changeIDs, nil
}

// handleDeleteChange processes a delete change by sending a delete update to the channel.
func (d *GrovePostgresDriver) handleDeleteChange(change sqlc.GetPortalAppChangesRow) {
	// Send the delete update
	d.updatesCh <- store.PortalAppUpdate{
		PortalAppID: store.PortalAppID(change.PortalAppID),
		Delete:      true,
	}
}

// handleUpsertChange processes an upsert change by fetching the portal app details
// and sending an update to the channel.
func (d *GrovePostgresDriver) handleUpsertChange(ctx context.Context, qtx *sqlc.Queries, change sqlc.GetPortalAppChangesRow) {
	// Get the portal app details
	portalAppRow, err := qtx.SelectPortalApp(ctx, change.PortalAppID)
	if err == nil {
		portalApp := sqlcPortalAppToPortalAppRow(portalAppRow).convertToPortalApp()

		// Send the upsert update
		d.updatesCh <- store.PortalAppUpdate{
			PortalAppID: portalApp.ID,
			PortalApp:   portalApp,
		}
	}
}

// markChangesAsProcessed marks the specified changes as processed in the database.
// This prevents them from being processed again.
func (d *GrovePostgresDriver) markChangesAsProcessed(ctx context.Context, qtx *sqlc.Queries, changeIDs []int32) error {
	if len(changeIDs) == 0 {
		return nil
	}

	err := qtx.MarkPortalAppChangesProcessed(ctx, changeIDs)
	if err != nil {
		return fmt.Errorf("failed to mark changes as processed: %w", err)
	}

	d.logger.Debug().Int("processed_count", len(changeIDs)).Msg("Processed portal application changes")
	return nil
}
