package ratelimit

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/bigquery"
	"github.com/buildwithgrove/path-external-auth-server/store"
	"google.golang.org/api/iterator"
)

const (
	relaysTable = "relays"
)

// dataWarehouseDriver handles BigQuery operations for data warehouse queries
type dataWarehouseDriver struct {
	clientBQ  *bigquery.Client
	projectID string
}

// monthlyUsageRow represents a row from the monthly usage query
type monthlyUsageRow struct {
	AccountID string `bigquery:"account_id"`
	Success   int64  `bigquery:"success"`
	Failure   int64  `bigquery:"failure"`
}

// ===========================================================================================
// Constructor and Cleanup
// ===========================================================================================

// newDriver creates a new BigQuery dataWarehouseDriver instance
func newDriver(ctx context.Context, projectID string) (*dataWarehouseDriver, error) {
	clientBQ, err := bigquery.NewClient(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to bigQuery: %w", err)
	}

	return &dataWarehouseDriver{
		clientBQ:  clientBQ,
		projectID: projectID,
	}, nil
}

// close releases BigQuery client resources
func (d *dataWarehouseDriver) close() {
	d.clientBQ.Close()
}

// ===========================================================================================
// Main Entry Point
// ===========================================================================================

// getMonthToMomentUsage returns monthly usage totals for accounts exceeding the threshold.
// Returns a map of account_id -> total relay count (success + failure) for accounts
// with monthly usage greater than 150,000 relays.
func (d *dataWarehouseDriver) getMonthToMomentUsage(ctx context.Context) (map[store.AccountID]int64, error) {
	// Execute query
	query := getMonthlyUsageQuery(d.projectID)
	it, err := d.clientBQ.Query(query).Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to execute monthly usage query: %w", err)
	}

	// Process results
	results := make(map[store.AccountID]int64)
	for {
		var row monthlyUsageRow
		err := it.Next(&row)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read query result: %w", err)
		}

		totalUsage := row.Success + row.Failure
		results[store.AccountID(row.AccountID)] = totalUsage
	}

	return results, nil
}

// ===========================================================================================
// Query Builders
// ===========================================================================================

// TODO_IN_THIS_PR(@commoddity): Determine the best method for getting month to date usage.
//   - Either we need a new table that aggregates usage month to date
//     that we can query directly for month-to-moment usage
//   - Or we can use an existing table and aggregate the month-to-moment usage ourselves
//   - Third option?
//
//   Would also be great if we could filter only accounts that are either:
//     - PlanType is PLAN_FREE
//     - PlanType is PLAN_UNLIMITED and MonthlyUserLimit is set (ie greater than 0)
//
// Use whichever method is the fastest and cheapest for frequent querying.

// getMonthlyUsageQuery builds the BigQuery SQL for monthly usage aggregation
func getMonthlyUsageQuery(projectID string) string {
	now := time.Now()
	firstOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	return fmt.Sprintf(`
		SELECT
			account_id,
			SUM(txs_cnt) AS success,
			SUM(errs_cnt) AS failure
		FROM
			`+"`%s.API.%s`"+`
		WHERE
			ts >= '%s'
			AND ts < '%s'
		GROUP BY
			account_id
		ORDER BY
			account_id;
	`, projectID, relaysTable,
		firstOfMonth.Format(time.RFC3339),
		now.Format(time.RFC3339))
}
