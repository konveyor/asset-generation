package discoverers

import (
	pTypes "github.com/konveyor/asset-generation/pkg/providers/types/provider"
)

// Provider defines the contract for application discovery providers.
// Implementations should support discovering applications from various data sources
// and listing available applications.
type Provider interface {
	// Discover extracts application information from raw data sources
	// and returns structured results including both public content and sensitive data.
	Discover(RawData any) (*pTypes.DiscoverResult, error)
	// ListApps returns a map of available applications
	ListApps() (map[string][]any, error)
}
