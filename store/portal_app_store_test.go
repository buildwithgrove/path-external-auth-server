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
		update                 *PortalAppUpdate
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
			name:                   "should return brand new portal app when update is received for new portal",
			portalAppID:            "portal_app_3_static_key",
			update:                 getTestUpdate("portal_app_3_static_key"),
			expectedPortalApp:      getTestUpdate("portal_app_3_static_key").PortalApp,
			expectedPortalAppFound: true,
		},
		{
			name:                   "should return updated existing portal app when update is received for existing portal",
			portalAppID:            "portal_app_2_no_auth",
			update:                 getTestUpdate("portal_app_2_no_auth"),
			expectedPortalApp:      getTestUpdate("portal_app_2_no_auth").PortalApp,
			expectedPortalAppFound: true,
		},
		{
			name:                   "should not return portal app when update is received to delete portal",
			portalAppID:            "portal_app_1_static_key",
			update:                 getTestUpdate("portal_app_1_static_key"),
			expectedPortalApp:      nil,
			expectedPortalAppFound: false,
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
			// Create channel for updates
			updates := make(chan PortalAppUpdate, 10)

			// Set up expectations
			mockDS.EXPECT().FetchInitialData().Return(getTestPortalApps(), nil).AnyTimes()
			mockDS.EXPECT().GetUpdateChannel().Return(updates).AnyTimes()
			mockDS.EXPECT().Close().AnyTimes()

			// Create store
			store, err := NewPortalAppStore(polyzero.NewLogger(), mockDS)
			c.NoError(err)

			// Send updates for this test case
			if test.update != nil {
				updates <- *test.update
				// Allow time for update to be processed
				time.Sleep(50 * time.Millisecond)
			}

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

			// Close the channel to end the test cleanly
			close(updates)
		})
	}
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

// getTestUpdate returns a mock update for a given portal app ID, used to test the portal app store's behavior when updates are received.
// Will be one of three cases:
// 1. An existing PortalApp was updated (portal_app_2_no_auth)
// 2. A new PortalApp was created (portal_app_3_static_key)
// 3. An existing PortalApp was deleted (portal_app_1_static_key)
func getTestUpdate(portalAppID string) *PortalAppUpdate {
	updatesMap := map[string]*PortalAppUpdate{
		"portal_app_2_no_auth": {
			PortalAppID: "portal_app_2_no_auth",
			PortalApp: &PortalApp{
				ID:        "portal_app_2_no_auth",
				AccountID: "account_2",
				Auth:      nil,
				RateLimit: &RateLimit{
					PlanType:         "PLAN_UNLIMITED",
					MonthlyUserLimit: 0,
				},
			},
			Delete: false,
		},
		"portal_app_3_static_key": {
			PortalAppID: "portal_app_3_static_key",
			PortalApp: &PortalApp{
				ID:        "portal_app_3_static_key",
				AccountID: "account_3",
				Auth: &Auth{
					APIKey: "new_api_key",
				},
				RateLimit: &RateLimit{
					PlanType:         "PLAN_PRO",
					MonthlyUserLimit: 1000,
				},
			},
			Delete: false,
		},
		"portal_app_1_static_key": {
			PortalAppID: "portal_app_1_static_key",
			PortalApp:   nil,
			Delete:      true,
		},
	}

	return updatesMap[portalAppID]
}
