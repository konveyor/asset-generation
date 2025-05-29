package providers

import (
	pTypes "github.com/konveyor/asset-generation/pkg/providers/types/provider"
)

type Provider interface {
	Discover() (result pTypes.DiscoverResult, err error)
	ListApps() ([]string, error)
}
