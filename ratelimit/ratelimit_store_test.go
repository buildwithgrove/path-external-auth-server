package ratelimit

import (
	"errors"
	"testing"
	"time"

	"github.com/pokt-network/poktroll/pkg/polylog/polyzero"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	grovedb "github.com/buildwithgrove/path-external-auth-server/postgres/grove"
	"github.com/buildwithgrove/path-external-auth-server/store"
)

func TestNewRateLimitStore(t *testing.T) {
	tests := []struct {
		name                    string
		setupMocks              func(*MockdataWarehouseDriver, *MockaccountPortalAppStore)
		expectError             bool
		expectedInitialUpdate   bool
		rateLimitUpdateInterval time.Duration
	}{
		{
			name: "should create rate limit store successfully with successful initial update",
			setupMocks: func(mockDWH *MockdataWarehouseDriver, mockAccountStore *MockaccountPortalAppStore) {
				mockDWH.EXPECT().
					GetMonthToMomentUsage(gomock.Any(), int64(FreeMonthlyRelays)).
					Return(map[string]int64{}, nil)
			},
			expectError:             false,
			expectedInitialUpdate:   true,
			rateLimitUpdateInterval: 1 * time.Minute,
		},
		{
			name: "should create rate limit store successfully even with failed initial update",
			setupMocks: func(mockDWH *MockdataWarehouseDriver, mockAccountStore *MockaccountPortalAppStore) {
				mockDWH.EXPECT().
					GetMonthToMomentUsage(gomock.Any(), int64(FreeMonthlyRelays)).
					Return(nil, errors.New("dwh connection failed"))
			},
			expectError:             false,
			expectedInitialUpdate:   false,
			rateLimitUpdateInterval: 1 * time.Minute,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := require.New(t)

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockDWH := NewMockdataWarehouseDriver(ctrl)
			mockAccountStore := NewMockaccountPortalAppStore(ctrl)

			test.setupMocks(mockDWH, mockAccountStore)

			rls, err := NewRateLimitStore(
				polyzero.NewLogger(),
				mockDWH,
				mockAccountStore,
				test.rateLimitUpdateInterval,
			)

			if test.expectError {
				c.Error(err)
				c.Nil(rls)
			} else {
				c.NoError(err)
				c.NotNil(rls)
				c.NotNil(rls.rateLimitedAccounts)
				c.NotNil(rls.logger)
				c.Equal(mockDWH, rls.dataWarehouseDriver)
				c.Equal(mockAccountStore, rls.accountPortalAppStore)
			}
		})
	}
}

func TestIsAccountRateLimited(t *testing.T) {
	tests := []struct {
		name                  string
		accountID             store.AccountID
		rateLimitedAccounts   map[store.AccountID]bool
		expectedIsRateLimited bool
	}{
		{
			name:      "should return true if account is rate limited",
			accountID: "rate_limited_account",
			rateLimitedAccounts: map[store.AccountID]bool{
				"rate_limited_account": true,
			},
			expectedIsRateLimited: true,
		},
		{
			name:                  "should return false if account is not rate limited",
			accountID:             "normal_account",
			rateLimitedAccounts:   map[store.AccountID]bool{},
			expectedIsRateLimited: false,
		},
		{
			name:      "should return false if account is not in rate limited map",
			accountID: "another_account",
			rateLimitedAccounts: map[store.AccountID]bool{
				"rate_limited_account": true,
			},
			expectedIsRateLimited: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := require.New(t)

			rls := &rateLimitStore{
				rateLimitedAccounts: test.rateLimitedAccounts,
			}

			result := rls.IsAccountRateLimited(test.accountID)
			c.Equal(test.expectedIsRateLimited, result)
		})
	}
}

func TestUpdateRateLimitedAccounts(t *testing.T) {
	tests := []struct {
		name                     string
		setupMocks               func(*MockdataWarehouseDriver, *MockaccountPortalAppStore)
		expectedRateLimitedCount int
		expectError              bool
	}{
		{
			name: "should update rate limited accounts with free plan account over limit",
			setupMocks: func(mockDWH *MockdataWarehouseDriver, mockAccountStore *MockaccountPortalAppStore) {
				usageData := map[string]int64{
					"free_account_over_limit": FreeMonthlyRelays + 1000,
				}
				mockDWH.EXPECT().
					GetMonthToMomentUsage(gomock.Any(), int64(FreeMonthlyRelays)).
					Return(usageData, nil)

				mockAccountStore.EXPECT().
					GetAccountPortalApp(store.AccountID("free_account_over_limit")).
					Return(&store.PortalApp{
						PlanType: grovedb.PlanFree_DatabaseType,
						RateLimit: &store.RateLimit{
							MonthlyUserLimit: 0,
						},
					}, true)
			},
			expectedRateLimitedCount: 1,
			expectError:              false,
		},
		{
			name: "should not rate limit free plan account under limit",
			setupMocks: func(mockDWH *MockdataWarehouseDriver, mockAccountStore *MockaccountPortalAppStore) {
				usageData := map[string]int64{
					"free_account_under_limit": FreeMonthlyRelays - 1000,
				}
				mockDWH.EXPECT().
					GetMonthToMomentUsage(gomock.Any(), int64(FreeMonthlyRelays)).
					Return(usageData, nil)

				mockAccountStore.EXPECT().
					GetAccountPortalApp(store.AccountID("free_account_under_limit")).
					Return(&store.PortalApp{
						PlanType: grovedb.PlanFree_DatabaseType,
						RateLimit: &store.RateLimit{
							MonthlyUserLimit: 0,
						},
					}, true)
			},
			expectedRateLimitedCount: 0,
			expectError:              false,
		},
		{
			name: "should rate limit unlimited plan account over custom limit",
			setupMocks: func(mockDWH *MockdataWarehouseDriver, mockAccountStore *MockaccountPortalAppStore) {
				usageData := map[string]int64{
					"unlimited_account_over_custom_limit": 500_000,
				}
				mockDWH.EXPECT().
					GetMonthToMomentUsage(gomock.Any(), int64(FreeMonthlyRelays)).
					Return(usageData, nil)

				mockAccountStore.EXPECT().
					GetAccountPortalApp(store.AccountID("unlimited_account_over_custom_limit")).
					Return(&store.PortalApp{
						PlanType: grovedb.PlanUnlimited_DatabaseType,
						RateLimit: &store.RateLimit{
							MonthlyUserLimit: 400_000,
						},
					}, true)
			},
			expectedRateLimitedCount: 1,
			expectError:              false,
		},
		{
			name: "should not rate limit unlimited plan account with no limit set",
			setupMocks: func(mockDWH *MockdataWarehouseDriver, mockAccountStore *MockaccountPortalAppStore) {
				usageData := map[string]int64{
					"unlimited_account_no_limit": FreeMonthlyRelays,
				}
				mockDWH.EXPECT().
					GetMonthToMomentUsage(gomock.Any(), int64(FreeMonthlyRelays)).
					Return(usageData, nil)

				mockAccountStore.EXPECT().
					GetAccountPortalApp(store.AccountID("unlimited_account_no_limit")).
					Return(&store.PortalApp{
						PlanType: grovedb.PlanUnlimited_DatabaseType,
						RateLimit: &store.RateLimit{
							MonthlyUserLimit: 0, // No limit set
						},
					}, true)
			},
			expectedRateLimitedCount: 0,
			expectError:              false,
		},
		{
			name: "should skip accounts without rate limit configuration",
			setupMocks: func(mockDWH *MockdataWarehouseDriver, mockAccountStore *MockaccountPortalAppStore) {
				usageData := map[string]int64{
					"account_without_config": 500_000,
				}
				mockDWH.EXPECT().
					GetMonthToMomentUsage(gomock.Any(), int64(FreeMonthlyRelays)).
					Return(usageData, nil)

				mockAccountStore.EXPECT().
					GetAccountPortalApp(store.AccountID("account_without_config")).
					Return(nil, false)
			},
			expectedRateLimitedCount: 0,
			expectError:              false,
		},
		{
			name: "should handle unknown plan types gracefully",
			setupMocks: func(mockDWH *MockdataWarehouseDriver, mockAccountStore *MockaccountPortalAppStore) {
				usageData := map[string]int64{
					"account_unknown_plan": 500_000,
				}
				mockDWH.EXPECT().
					GetMonthToMomentUsage(gomock.Any(), int64(FreeMonthlyRelays)).
					Return(usageData, nil)

				mockAccountStore.EXPECT().
					GetAccountPortalApp(store.AccountID("account_unknown_plan")).
					Return(&store.PortalApp{
						PlanType: "PLAN_UNKNOWN",
						RateLimit: &store.RateLimit{
							MonthlyUserLimit: 100_000,
						},
					}, true)
			},
			expectedRateLimitedCount: 0,
			expectError:              false,
		},
		{
			name: "should return error when data warehouse fails",
			setupMocks: func(mockDWH *MockdataWarehouseDriver, mockAccountStore *MockaccountPortalAppStore) {
				mockDWH.EXPECT().
					GetMonthToMomentUsage(gomock.Any(), int64(FreeMonthlyRelays)).
					Return(nil, errors.New("data warehouse connection failed"))
			},
			expectedRateLimitedCount: 0,
			expectError:              true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := require.New(t)

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockDWH := NewMockdataWarehouseDriver(ctrl)
			mockAccountStore := NewMockaccountPortalAppStore(ctrl)

			test.setupMocks(mockDWH, mockAccountStore)

			rls := &rateLimitStore{
				logger:                polyzero.NewLogger(),
				dataWarehouseDriver:   mockDWH,
				accountPortalAppStore: mockAccountStore,
				rateLimitedAccounts:   make(map[store.AccountID]bool),
			}

			err := rls.updateRateLimitedAccounts()

			if test.expectError {
				c.Error(err)
			} else {
				c.NoError(err)
				c.Equal(test.expectedRateLimitedCount, len(rls.rateLimitedAccounts))
			}
		})
	}
}

func TestShouldLimitAccount(t *testing.T) {
	tests := []struct {
		name           string
		rateLimit      store.RateLimit
		planType       store.PlanType
		usage          int64
		expectedResult bool
	}{
		{
			name: "should limit free plan account over free tier limit",
			rateLimit: store.RateLimit{
				MonthlyUserLimit: 0,
			},
			planType:       grovedb.PlanFree_DatabaseType,
			usage:          FreeMonthlyRelays + 1000,
			expectedResult: true,
		},
		{
			name: "should not limit free plan account under free tier limit",
			rateLimit: store.RateLimit{
				MonthlyUserLimit: 0,
			},
			planType:       grovedb.PlanFree_DatabaseType,
			usage:          FreeMonthlyRelays - 1000,
			expectedResult: false,
		},
		{
			name: "should limit free plan account exactly at free tier limit",
			rateLimit: store.RateLimit{
				MonthlyUserLimit: 0,
			},
			planType:       grovedb.PlanFree_DatabaseType,
			usage:          FreeMonthlyRelays,
			expectedResult: false, // > comparison, so exactly at limit is not limited
		},
		{
			name: "should limit unlimited plan account over custom limit",
			rateLimit: store.RateLimit{
				MonthlyUserLimit: 500_000,
			},
			planType:       grovedb.PlanUnlimited_DatabaseType,
			usage:          600_000,
			expectedResult: true,
		},
		{
			name: "should not limit unlimited plan account under custom limit",
			rateLimit: store.RateLimit{
				MonthlyUserLimit: 500_000,
			},
			planType:       grovedb.PlanUnlimited_DatabaseType,
			usage:          400_000,
			expectedResult: false,
		},
		{
			name: "should not limit unlimited plan account with no custom limit",
			rateLimit: store.RateLimit{
				MonthlyUserLimit: 0,
			},
			planType:       grovedb.PlanUnlimited_DatabaseType,
			usage:          FreeMonthlyRelays,
			expectedResult: false,
		},
		{
			name: "should not limit unknown plan type",
			rateLimit: store.RateLimit{
				MonthlyUserLimit: 100_000,
			},
			planType:       "PLAN_UNKNOWN",
			usage:          200_000,
			expectedResult: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := require.New(t)

			rls := &rateLimitStore{
				logger: polyzero.NewLogger(),
			}

			result := rls.shouldLimitAccount(test.rateLimit, test.planType, test.usage)
			c.Equal(test.expectedResult, result)
		})
	}
}

func TestRateLimitStoreIntegration(t *testing.T) {
	t.Run("should handle complete rate limiting workflow", func(t *testing.T) {
		c := require.New(t)

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockDWH := NewMockdataWarehouseDriver(ctrl)
		mockAccountStore := NewMockaccountPortalAppStore(ctrl)

		// Setup initial data - one account over limit, one under
		initialUsageData := map[string]int64{
			"free_account_over":  FreeMonthlyRelays + 5000,
			"free_account_under": FreeMonthlyRelays - 5000,
			"unlimited_account":  500_000,
		}

		// First call during NewRateLimitStore
		mockDWH.EXPECT().
			GetMonthToMomentUsage(gomock.Any(), int64(FreeMonthlyRelays)).
			Return(initialUsageData, nil)

		mockAccountStore.EXPECT().
			GetAccountPortalApp(store.AccountID("free_account_over")).
			Return(&store.PortalApp{
				PlanType: grovedb.PlanFree_DatabaseType,
				RateLimit: &store.RateLimit{
					MonthlyUserLimit: 0,
				},
			}, true)

		mockAccountStore.EXPECT().
			GetAccountPortalApp(store.AccountID("free_account_under")).
			Return(&store.PortalApp{
				PlanType: grovedb.PlanFree_DatabaseType,
				RateLimit: &store.RateLimit{
					MonthlyUserLimit: 0,
				},
			}, true)

		mockAccountStore.EXPECT().
			GetAccountPortalApp(store.AccountID("unlimited_account")).
			Return(&store.PortalApp{
				PlanType: grovedb.PlanUnlimited_DatabaseType,
				RateLimit: &store.RateLimit{
					MonthlyUserLimit: 0, // No limit
				},
			}, true)

		rls, err := NewRateLimitStore(
			polyzero.NewLogger(),
			mockDWH,
			mockAccountStore,
			1*time.Minute,
		)
		c.NoError(err)
		c.NotNil(rls)

		// Verify initial state
		c.True(rls.IsAccountRateLimited("free_account_over"))
		c.False(rls.IsAccountRateLimited("free_account_under"))
		c.False(rls.IsAccountRateLimited("unlimited_account"))
		c.False(rls.IsAccountRateLimited("nonexistent_account"))
	})
}

func TestRateLimitStoreConcurrency(t *testing.T) {
	t.Run("should handle concurrent access to rate limited accounts map", func(t *testing.T) {
		c := require.New(t)

		rls := &rateLimitStore{
			rateLimitedAccounts: map[store.AccountID]bool{
				"test_account": true,
			},
		}

		// Start multiple goroutines reading the map
		done := make(chan bool, 10)
		for i := 0; i < 10; i++ {
			go func() {
				for j := 0; j < 100; j++ {
					rls.IsAccountRateLimited("test_account")
					rls.IsAccountRateLimited("nonexistent_account")
				}
				done <- true
			}()
		}

		// Start one goroutine writing to the map
		go func() {
			for i := 0; i < 10; i++ {
				rls.rateLimitedAccountsMu.Lock()
				rls.rateLimitedAccounts = map[store.AccountID]bool{
					"new_account": true,
				}
				rls.rateLimitedAccountsMu.Unlock()
				time.Sleep(1 * time.Millisecond)
			}
			done <- true
		}()

		// Wait for all goroutines to complete
		for i := 0; i < 11; i++ {
			<-done
		}

		// Verify final state
		c.False(rls.IsAccountRateLimited("test_account"))
		c.True(rls.IsAccountRateLimited("new_account"))
	})
}
