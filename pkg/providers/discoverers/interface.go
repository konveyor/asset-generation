package discoverers

import (
	pTypes "github.com/konveyor/asset-generation/pkg/providers/types/provider"
)

type Provider interface {
	Discover(AppDiscoveryConfig any) (pTypes.DiscoverResult, error)
	ListApps() (map[string]any, error)
}
