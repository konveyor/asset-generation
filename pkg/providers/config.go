package providers

import (
	. "github.com/konveyor/asset-generation/pkg/providers/types/provider"
)

// Config is a marker interface for provider-specific configuration.
type Config interface {
	Type() ProviderType
}
