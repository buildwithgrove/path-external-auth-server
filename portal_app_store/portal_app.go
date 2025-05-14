package portalappstore

type (
	PortalAppID string
	AccountID   string
	PlanType    string
)

// PortalApp represents a single portal app for a user's account.
type PortalApp struct {
	// Unique identifier for the PortalApp.
	PortalAppID PortalAppID
	// Unique identifier for the PortalApp's account.
	AccountID AccountID
	// The authorization settings for the PortalApp.
	// Auth can be one of:
	//   - NoAuth: The portal app does not require authorization (Auth will be nil)
	//   - APIKey: The portal app uses an API key for authorization
	Auth *Auth
	// Rate Limiting settings for the PortalApp.
	// If the portal app is not rate limited, RateLimit will be nil.
	RateLimit *RateLimit
}

// Auth represents the authorization settings for a PortalApp.
// Only API key auth is supported by the Grove Portal.
type Auth struct {
	APIKey string
}

// RateLimit contains rate limiting settings for a PortalApp.
type RateLimit struct {
	PlanType         PlanType
	MonthlyUserLimit int32
}

// PortalAppUpdate represents an update to a portal app in the store
type PortalAppUpdate struct {
	// The ID of the portal app being updated
	PortalAppID PortalAppID
	// The new portal app data, nil if this is a deletion
	PortalApp *PortalApp
	// Whether this update is deleting the portal app
	Delete bool
}
