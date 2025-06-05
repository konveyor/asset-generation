package discoverers

import (
	pTypes "github.com/konveyor/asset-generation/pkg/providers/types/provider"
)

type Provider interface {
	Discover(RawData any) (*pTypes.DiscoverResult, error)
	ListApps() (map[string]any, error)
}
