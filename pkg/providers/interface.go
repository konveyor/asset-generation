package providers

import (
	pTypes "github.com/konveyor/asset-generation/pkg/providers/types/provider"
)

type Provider[T any] interface {
	GetProviderType() pTypes.ProviderType
	Discover(space string, appGUID string) (result T, err error)
	ListAppsBySpace(space string) ([]string, error)
}
