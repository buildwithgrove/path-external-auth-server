package postgrest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/pokt-network/poktroll/pkg/polylog"

	"github.com/buildwithgrove/path-external-auth-server/store"
	portal_db_sdk "github.com/grove/path/portal-db/sdk/go"
)

// PostgRESTDriver implements the store.DataSource interface
// to provide data from PostgREST API for the portal app store.
var _ store.DataSource = &PostgRESTDriver{}

type PostgRESTDriver struct {
	logger polylog.Logger

	client *portal_db_sdk.ClientWithResponses

	// JWT secret for generating authentication tokens
	jwtSecret string
	jwtRole   string
	jwtEmail  string

	timeout time.Duration
}

// NewPostgRESTDriver creates a new PostgREST driver that implements the store.DataSource interface.
//
// The driver connects to a PostgREST API and:
//  1. Provides methods to fetch initial portal app data
//  2. Uses JWT authentication for API requests
//  3. Converts PostgREST responses to store.PortalApp format
func NewPostgRESTDriver(
	logger polylog.Logger,
	baseURL string,
	jwtSecret string,
	jwtRole string,
	jwtEmail string,
	timeout time.Duration,
) (*PostgRESTDriver, error) {
	// Create PostgREST client with custom HTTP client
	client, err := portal_db_sdk.NewClientWithResponses(baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create PostgREST client: %w", err)
	}

	driver := &PostgRESTDriver{
		logger:    logger,
		client:    client,
		jwtSecret: jwtSecret,
		jwtRole:   jwtRole,
		jwtEmail:  jwtEmail,
		timeout:   timeout,
	}

	return driver, nil
}

// getRequestEditor generates a fresh JWT token and returns a request editor function
func (d *PostgRESTDriver) getRequestEditor(token string) (portal_db_sdk.RequestEditorFn, error) {
	// Return request editor with the generated token
	return func(ctx context.Context, req *http.Request) error {
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/json")
		return nil
	}, nil
}

// GetPortalApps loads the full set of PortalApps from the PostgREST API.
// It uses a two-step approach:
//  1. Fetch portal applications
//  2. Fetch portal accounts to get plan types
//  3. Merge the data and convert to store.PortalApp format
func (d *PostgRESTDriver) GetPortalApps() (map[store.PortalAppID]*store.PortalApp, error) {
	d.logger.Info().Msg("🌐 Executing PostgREST portal apps query...")

	// Step 1: Get portal applications
	applications, err := d.getPortalApplications()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch portal applications: %w", err)
	}

	if len(applications) == 0 {
		d.logger.Info().Msg("✅ No portal applications found")
		return make(map[store.PortalAppID]*store.PortalApp), nil
	}

	// Step 2: Get all portal accounts (we'll match them in Go code)
	accounts, err := d.getAllPortalAccounts()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch portal accounts: %w", err)
	}

	d.logger.Info().
		Int("num_applications", len(applications)).
		Int("num_accounts", len(accounts)).
		Msg("✅ Successfully fetched Portal Applications and Accounts from PostgREST")

	// Step 3: Convert and merge data
	return d.convertToPortalApps(applications, accounts), nil
}

// getPortalApplications fetches portal applications from PostgREST API
func (d *PostgRESTDriver) getPortalApplications() ([]portal_db_sdk.PortalApplications, error) {
	ctx, cancel := context.WithTimeout(context.Background(), d.timeout)
	defer cancel()

	// Build select fields for readability
	selectFields := strings.Join([]string{
		"portal_application_id",
		"portal_account_id",
		"secret_key_hash",
		"secret_key_required",
		"portal_application_user_limit",
	}, ",")

	params := &portal_db_sdk.GetPortalApplicationsParams{
		Select:    stringPtr(selectFields),
		DeletedAt: stringPtr("is.null"), // Only active applications
	}

	// Generate a fresh JWT token
	token, err := generatePostgRESTJWT(d.jwtSecret, d.jwtRole, d.jwtEmail)
	if err != nil {
		return nil, fmt.Errorf("failed to generate JWT token: %w", err)
	}

	requestEditor, err := d.getRequestEditor(token)
	if err != nil {
		return nil, fmt.Errorf("failed to create request editor: %w", err)
	}

	resp, err := d.client.GetPortalApplicationsWithResponse(ctx, params, requestEditor)
	if err != nil {
		d.logger.Error().Err(err).Msg("failed to call PostgREST portal applications endpoint")
		return nil, fmt.Errorf("failed to call PostgREST API: %w", err)
	}

	if resp.StatusCode() != 200 {
		d.logger.Error().Int("status_code", resp.StatusCode()).Msg("unexpected status code from PostgREST")
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode())
	}

	// Parse JSON response directly
	var applications []portal_db_sdk.PortalApplications
	if err := json.Unmarshal(resp.Body, &applications); err != nil {
		d.logger.Error().Err(err).Msg("failed to parse portal applications response")
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	return applications, nil
}

// getAllPortalAccounts fetches all non-deleted portal accounts with plan types from PostgREST API
func (d *PostgRESTDriver) getAllPortalAccounts() ([]portal_db_sdk.PortalAccounts, error) {
	ctx, cancel := context.WithTimeout(context.Background(), d.timeout)
	defer cancel()

	// Build select fields for minimal data transfer
	selectFields := strings.Join([]string{
		"portal_account_id",
		"portal_plan_type",
		"portal_account_user_limit",
	}, ",")

	params := &portal_db_sdk.GetPortalAccountsParams{
		Select:    stringPtr(selectFields),
		DeletedAt: stringPtr("is.null"), // Only active accounts
	}

	// Generate a fresh JWT token
	token, err := generatePostgRESTJWT(d.jwtSecret, d.jwtRole, d.jwtEmail)
	if err != nil {
		return nil, fmt.Errorf("failed to generate JWT token: %w", err)
	}

	requestEditor, err := d.getRequestEditor(token)
	if err != nil {
		return nil, fmt.Errorf("failed to create request editor: %w", err)
	}

	resp, err := d.client.GetPortalAccountsWithResponse(ctx, params, requestEditor)
	if err != nil {
		d.logger.Error().Err(err).Msg("failed to call PostgREST portal accounts endpoint")
		return nil, fmt.Errorf("failed to call PostgREST API: %w", err)
	}

	if resp.StatusCode() != 200 {
		d.logger.Error().Int("status_code", resp.StatusCode()).Msg("unexpected status code from PostgREST")
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode())
	}

	// Parse JSON response directly
	var accounts []portal_db_sdk.PortalAccounts
	if err := json.Unmarshal(resp.Body, &accounts); err != nil {
		d.logger.Error().Err(err).Msg("failed to parse portal accounts response")
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	return accounts, nil
}

// buildAccountPlanMap creates a map of account ID to plan type for efficient lookups
func (d *PostgRESTDriver) buildAccountMap(accounts []portal_db_sdk.PortalAccounts) map[string]portal_db_sdk.PortalAccounts {
	accountsMap := make(map[string]portal_db_sdk.PortalAccounts, len(accounts))
	for _, account := range accounts {
		accountsMap[account.PortalAccountId] = account
	}
	return accountsMap
}

// convertToPortalApps converts PostgREST responses to store.PortalApp format
func (d *PostgRESTDriver) convertToPortalApps(
	applications []portal_db_sdk.PortalApplications,
	accounts []portal_db_sdk.PortalAccounts,
) map[store.PortalAppID]*store.PortalApp {
	// Build efficient lookup map for account plan types
	accountsMap := d.buildAccountMap(accounts)

	// Convert applications to PortalApps
	portalApps := make(map[store.PortalAppID]*store.PortalApp, len(applications))

	for _, app := range applications {
		// Get plan type from account mapping
		account, exists := accountsMap[app.PortalAccountId]
		if !exists {
			d.logger.Warn().
				Str("portal_application_id", app.PortalApplicationId).
				Str("portal_account_id", app.PortalAccountId).
				Msg("⁉️ SHOULD NEVER HAPPEN - No plan type found for account, skipping application")
			continue
		}

		// Convert SDK types directly to store.PortalApp
		portalApp := convertSDKToPortalApp(app, account)

		portalApps[store.PortalAppID(app.PortalApplicationId)] = portalApp
	}

	return portalApps
}

// Close is a no-op for the PostgREST driver.
// TODO_TECHDEBT(@commoddity): Remove this method when direct Postgres data source is removed.
func (d *PostgRESTDriver) Close() {
	// Do nothing; only here to satisfy the DataSource interface
}

// stringPtr is a helper function to create string pointers
func stringPtr(s string) *string {
	return &s
}
