package providers

import (
	pTypes "github.com/konveyor/asset-generation/pkg/providers/types/provider"
)

type Provider[T any] interface {
	GetProviderType() pTypes.ProviderType
	Discover() (result T, err error)
}
