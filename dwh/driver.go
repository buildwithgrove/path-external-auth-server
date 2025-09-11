package dwh

import (
	"context"
	"fmt"

	"cloud.google.com/go/bigquery"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

// Driver handles BigQuery operations for data warehouse queries
type Driver struct {
	clientBQ  *bigquery.Client
	projectID string
}

// monthlyUsageRow represents a row from the monthly usage query
type monthlyUsageRow struct {
	AccountID   string `bigquery:"account_id"`
	TotalRelays int64  `bigquery:"total_relays"`
}

// ===========================================================================================
// Constructor and Cleanup
// ===========================================================================================

// NewDriver creates a new BigQuery Driver instance.
// If GOOGLE_APPLICATION_CREDENTIALS_FILE environment variable is set,
// it will use that JSON credentials file for authentication.
// Otherwise, it falls back to default credentials (ADC).
func NewDriver(ctx context.Context, projectID string, credsFile string) (*Driver, error) {
	clientBQ, err := bigquery.NewClient(ctx, projectID, option.WithCredentialsFile(credsFile))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to bigQuery: %w", err)
	}

	return &Driver{
		clientBQ:  clientBQ,
		projectID: projectID,
	}, nil
}

// close releases BigQuery client resources
func (d *Driver) Close() {
	d.clientBQ.Close()
}

// ===========================================================================================
// Main Entry Point
// ===========================================================================================

// GetMonthToMomentUsage returns monthly usage totals for accounts above the threshold.
//
// The query aggregates relay counts from the first day of the current month through today,
// grouped by account_id. Only returns accounts with relay activity above minRelayThreshold.
//
// Returns a map of account_id -> total relay count for month-to-date usage.
func (d *Driver) GetMonthToMomentUsage(
	ctx context.Context,
	minRelayThreshold int64,
) (map[string]int64, error) {
	// Execute query with project ID and threshold
	query := getMonthlyUsageQuery(d.projectID, minRelayThreshold)
	it, err := d.clientBQ.Query(query).Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to execute monthly usage query: %w", err)
	}

	// Process results
	results := make(map[string]int64)
	for {
		var row monthlyUsageRow
		err := it.Next(&row)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read query result: %w", err)
		}

		results[row.AccountID] = row.TotalRelays
	}

	return results, nil
}

// getMonthlyUsageQuery returns the BigQuery SQL for monthly usage aggregation.
//
// The query performs month-to-date filtering using BigQuery's date functions:
// - DATE_TRUNC(CURRENT_DATE(), MONTH) gets the first day of current month
// - CURRENT_DATE() gets today's date
// - Only includes accounts above the specified relay threshold
// - Results are ordered by total relay count (highest first)
//
// Parameters:
// - projectID: GCP project containing the dataset
// - minRelayThreshold: minimum relay count to include accounts
func getMonthlyUsageQuery(
	projectID string,
	minRelayThreshold int64,
) string {
	return fmt.Sprintf(`
		SELECT
			account_id,
			SUM(COALESCE(txs_cnt, 0) + COALESCE(errs_cnt, 0)) AS total_relays
		FROM
			`+"`%s.API.relays`"+`
		WHERE
			DATE(ts) >= DATE_TRUNC(CURRENT_DATE(), MONTH)
			AND DATE(ts) <= CURRENT_DATE()
			AND account_id IS NOT NULL
		GROUP BY
			account_id
		HAVING
			SUM(COALESCE(txs_cnt, 0) + COALESCE(errs_cnt, 0)) >= %d
		ORDER BY
			total_relays DESC, account_id;
	`, projectID, minRelayThreshold)
}
