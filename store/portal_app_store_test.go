package store

import (
	"testing"

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

			// Set up expectations - only need FetchInitialData for periodic refresh
			mockDS.EXPECT().FetchInitialData().Return(getTestPortalApps(), nil).AnyTimes()

			// Create store
			store, err := NewPortalAppStore(polyzero.NewLogger(), mockDS)
			c.NoError(err)
			defer store.Stop()

			// Test GetPortalApp
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
					c.Equal(test.expectedPortalApp.RateLimit.PlanType, portalApp.RateLimit.PlanType)
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

func Test_PeriodicRefresh(t *testing.T) {
	c := require.New(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create mock data source
	mockDS := NewMockDataSource(ctrl)

	// Initial data
	initialData := getTestPortalApps()

	// Set up expectations - only expect the initial call since we can't easily test
	// the 30-second periodic refresh without making the interval configurable
	mockDS.EXPECT().FetchInitialData().Return(initialData, nil).Times(1)

	// Create store
	store, err := NewPortalAppStore(polyzero.NewLogger(), mockDS)
	c.NoError(err)
	defer store.Stop()

	// Verify initial state
	portalApp1, found1 := store.GetPortalApp("portal_app_1_static_key")
	c.True(found1)
	c.Equal(PortalAppID("portal_app_1_static_key"), portalApp1.ID)

	portalApp2, found2 := store.GetPortalApp("portal_app_2_no_auth")
	c.True(found2)
	c.Equal(PortalAppID("portal_app_2_no_auth"), portalApp2.ID)

	portalApp3, found3 := store.GetPortalApp("portal_app_3_new")
	c.False(found3)
	c.Nil(portalApp3)

	// TODO_TECHDEBT(@adshmh): Testing the actual periodic refresh (30-second timer) would require:
	// 1. Making the refresh interval configurable for testing, or
	// 2. Dependency injection for the ticker, or
	// 3. More complex mocking setup
	//
	// For now, this test verifies the store initializes correctly and basic
	// GetPortalApp functionality works as expected.
}

// getTestPortalApps returns a mock response for the initial portal app store data,
// received when the portal app store is first created.
func getTestPortalApps() map[PortalAppID]*PortalApp {
	return map[PortalAppID]*PortalApp{
		"portal_app_1_static_key": {
			ID:        "portal_app_1_static_key",
			AccountID: "account_1",
			Auth: &Auth{
				APIKey: "api_key_1",
			},
			RateLimit: &RateLimit{
				PlanType:         "PLAN_UNLIMITED",
				MonthlyUserLimit: 0,
			},
		},
		"portal_app_2_no_auth": {
			ID:        "portal_app_2_no_auth",
			AccountID: "account_2",
			Auth:      nil,
			RateLimit: &RateLimit{
				PlanType:         "PLAN_UNLIMITED",
				MonthlyUserLimit: 0,
			},
		},
	}
}
