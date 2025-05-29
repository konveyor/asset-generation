package providers

import (
	"fmt"
	"io"
	"log"

	cfProvider "github.com/konveyor/asset-generation/pkg/providers/cloud_foundry"
	providerTypes "github.com/konveyor/asset-generation/pkg/providers/types/provider"
)

func NewProvider(cfg Config, logger *log.Logger) (Provider, error) {
	if logger == nil {
		logger = log.New(io.Discard, "", log.LstdFlags) // No-op logger
	}
	switch cfg.Type() {
	case providerTypes.ProviderTypeCloudFoundry:
		logger.Println("Creating new CF provider")
		cfCfg, ok := cfg.(*cfProvider.Config)
		if !ok {
			return nil, fmt.Errorf("invalid config type for cf")
		}
		return cfProvider.New(cfCfg, logger), nil
	default:
		return nil, fmt.Errorf("unsupported provider type: %s", cfg.Type())
	}
}
