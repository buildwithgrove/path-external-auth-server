package portalappstore

type (
	PortalAppID string
	AccountID   string
	PlanType    string
)

// PortalApp represents a single portal app for a user's account.
type PortalApp struct {
	// Used to identify the PortalApp when making a service request.
	PortalAppID PortalAppID
	// Unique identifier for the user's account
	AccountID AccountID
	// The authorization settings for the PortalApp.
	Auth *Auth
	// Rate Limiting settings for the PortalApp.
	RateLimit *RateLimit
}

// Auth represents the authorization settings for a PortalApp.
// Auth can be one of:
//   - NoAuth: The portal app does not require authorization (no fields are set)
//   - APIKey: The portal app uses an API key for authorization
type Auth struct {
	APIKey string
}

// RateLimit contains rate limiting settings for a PortalApp.
type RateLimit struct {
	PlanType         PlanType
	MonthlyUserLimit int32
}

// PortalAppUpdate represents an update to a gateway portal app in the store
type PortalAppUpdate struct {
	// The ID of the portal app being updated
	PortalAppID PortalAppID
	// The new portal app data, nil if this is a deletion
	PortalApp *PortalApp
	// Whether this update is deleting the portal app
	Delete bool
}
