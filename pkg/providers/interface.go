package providers

import (
	dTypes "github.com/konveyor/asset-generation/pkg/providers/types/discover"
	pTypes "github.com/konveyor/asset-generation/pkg/providers/types/provider"
)

type Provider interface {
	GetProviderType() pTypes.ProviderType
	OffilineDiscover() ([]dTypes.Application, error)
	LiveDiscover(spaceNames []string) error
}
