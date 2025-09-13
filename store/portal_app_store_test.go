package store

import (
	"testing"
	"time"

	"github.com/pokt-network/poktroll/pkg/polylog/polyzero"
	"github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"
)

func Test_GetPortalApp(t *testing.T) {
	tests := []struct {
		name                   string
		portalAppID            PortalAppID
		expectedPortalApp      *PortalApp
		expectedPortalAppFound bool
	}{
		{
			name:                   "should return portal app when found",
			portalAppID:            "portal_app_1_static_key",
			expectedPortalApp:      getTestPortalApps()["portal_app_1_static_key"],
			expectedPortalAppFound: true,
		},
		{
			name:                   "should return different portal app when found",
			portalAppID:            "portal_app_2_no_auth",
			expectedPortalApp:      getTestPortalApps()["portal_app_2_no_auth"],
			expectedPortalAppFound: true,
		},
		{
			name:                   "should return false when portal app not found",
			portalAppID:            "portal_app_3_static_key",
			expectedPortalApp:      nil,
			expectedPortalAppFound: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := require.New(t)

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// Create mock data source
			mockDS := NewMockDataSource(ctrl)

			// Set up expectations for initial data load
			mockDS.EXPECT().GetPortalApps().Return(getTestPortalApps(), nil).Times(1)

			// Create store with a long refresh interval to avoid interference during test
			store, err := NewPortalAppStore(polyzero.NewLogger(), mockDS, 1*time.Hour)
			c.NoError(err)

			portalApp, found := store.GetPortalApp(test.portalAppID)
			c.Equal(test.expectedPortalAppFound, found)

			if test.expectedPortalApp != nil {
				c.Equal(test.expectedPortalApp.ID, portalApp.ID)

				// Compare Auth details if present
				if test.expectedPortalApp.Auth != nil {
					c.Equal(test.expectedPortalApp.Auth.APIKey, portalApp.Auth.APIKey)
				} else {
					c.Nil(portalApp.Auth)
				}

				// Compare RateLimit if present
				if test.expectedPortalApp.RateLimit != nil {
					c.Equal(test.expectedPortalApp.PlanType, portalApp.PlanType)
					c.Equal(test.expectedPortalApp.RateLimit.MonthlyUserLimit, portalApp.RateLimit.MonthlyUserLimit)
				} else {
					c.Nil(portalApp.RateLimit)
				}
			} else {
				c.Nil(portalApp)
			}
		})
	}
}

func Test_BackgroundRefresh(t *testing.T) {
	c := require.New(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create mock data source
	mockDS := NewMockDataSource(ctrl)

	// Initial data load
	initialApps := getTestPortalApps()
	mockDS.EXPECT().GetPortalApps().Return(initialApps, nil).Times(1)

	// Updated data that will be returned on refresh
	updatedApps := getUpdatedTestPortalApps()
	mockDS.EXPECT().GetPortalApps().Return(updatedApps, nil).MinTimes(1)

	// Create store with short refresh interval for testing
	refreshInterval := 100 * time.Millisecond
	store, err := NewPortalAppStore(polyzero.NewLogger(), mockDS, refreshInterval)
	c.NoError(err)

	// Verify initial state
	portalApp, found := store.GetPortalApp("portal_app_1_static_key")
	c.True(found)
	c.Equal("api_key_1", portalApp.Auth.APIKey)

	// Wait for at least one refresh cycle
	time.Sleep(refreshInterval + 50*time.Millisecond)

	// Verify the store was updated with new data
	portalApp, found = store.GetPortalApp("portal_app_1_static_key")
	c.True(found)
	c.Equal("updated_api_key_1", portalApp.Auth.APIKey)

	// Verify new app was added
	newApp, found := store.GetPortalApp("portal_app_3_static_key")
	c.True(found)
	c.Equal("new_api_key", newApp.Auth.APIKey)
}

// getTestPortalApps returns a mock response for the initial portal app store data,
// received when the portal app store is first created.
func getTestPortalApps() map[PortalAppID]*PortalApp {
	return map[PortalAppID]*PortalApp{
		"portal_app_1_static_key": {
			ID:        "portal_app_1_static_key",
			AccountID: "account_1",
			PlanType:  "PLAN_UNLIMITED",
			Auth: &Auth{
				APIKey: "api_key_1",
			},
			RateLimit: &RateLimit{
				MonthlyUserLimit: 0,
			},
		},
		"portal_app_2_no_auth": {
			ID:        "portal_app_2_no_auth",
			AccountID: "account_2",
			PlanType:  "PLAN_UNLIMITED",
			Auth:      nil,
			RateLimit: &RateLimit{
				MonthlyUserLimit: 0,
			},
		},
	}
}

// getUpdatedTestPortalApps returns updated portal app data to simulate a refresh
func getUpdatedTestPortalApps() map[PortalAppID]*PortalApp {
	return map[PortalAppID]*PortalApp{
		"portal_app_1_static_key": {
			ID:        "portal_app_1_static_key",
			AccountID: "account_1",
			PlanType:  "PLAN_UNLIMITED",
			Auth: &Auth{
				APIKey: "updated_api_key_1",
			},
			RateLimit: &RateLimit{
				MonthlyUserLimit: 0,
			},
		},
		"portal_app_2_no_auth": {
			ID:        "portal_app_2_no_auth",
			AccountID: "account_2",
			PlanType:  "PLAN_UNLIMITED",
			Auth:      nil,
			RateLimit: &RateLimit{
				MonthlyUserLimit: 0,
			},
		},
		"portal_app_3_static_key": {
			ID:        "portal_app_3_static_key",
			AccountID: "account_3",
			PlanType:  "PLAN_UNLIMITED",
			Auth: &Auth{
				APIKey: "new_api_key",
			},
			RateLimit: &RateLimit{
				MonthlyUserLimit: 1000,
			},
		},
	}
}
