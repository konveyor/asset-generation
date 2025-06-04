package discoverers

import (
	pTypes "github.com/konveyor/asset-generation/pkg/providers/types/provider"
)

type Provider interface {
	Discover(space string, app string) (result pTypes.DiscoverResult, err error)
	ListApps() (map[string]any, error)
}
