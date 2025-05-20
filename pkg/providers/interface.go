package providers

import (
	pTypes "github.com/konveyor/asset-generation/pkg/providers/types/provider"
)

type Provider interface {
	GetProviderType() pTypes.ProviderType
	Discover() (T, error)
}
