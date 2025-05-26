package providers

import (
	"fmt"
	"io"
	"log"

	cfProvider "github.com/konveyor/asset-generation/pkg/providers/cloud_foundry"
	korifiProvider "github.com/konveyor/asset-generation/pkg/providers/korifi"
	providerTypes "github.com/konveyor/asset-generation/pkg/providers/types/provider"
)

func NewProvider[T any](cfg Config, logger *log.Logger) (Provider[T], error) {
	if logger == nil {
		logger = log.New(io.Discard, "", log.LstdFlags) // No-op logger
	}
	switch cfg.Type() {
	case providerTypes.ProviderTypeCF:
		logger.Println("Creating new CF provider")
		cfCfg, ok := cfg.(*cfProvider.Config)
		if !ok {
			return nil, fmt.Errorf("invalid config type for cf")
		}
		return any(cfProvider.New[T](cfCfg, logger)).(Provider[T]), nil
	case providerTypes.ProviderTypeKorifi:
		logger.Println("Creating new Korifi provider")
		korifiCfg, ok := cfg.(*korifiProvider.Config)
		if !ok {
			return nil, fmt.Errorf("invalid config type for korifi")
		}
		return any(korifiProvider.New[T](korifiCfg, logger)).(Provider[T]), nil
	default:
		return nil, fmt.Errorf("unsupported provider type: %s", cfg.Type())
	}
}
