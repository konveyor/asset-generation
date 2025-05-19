package providers

import (
	. "github.com/konveyor/asset-generation/pkg/providers/types"
)

type Provider interface {
	GetProviderType() ProviderType
	OffilineDiscover() ([]Application, error)
	LiveDiscover(spaceNames []string) error
}
