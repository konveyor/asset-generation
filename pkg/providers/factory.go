package providers

import (
	"fmt"

	"github.com/konveyor/asset-generation/pkg/providers/cf"
	cfProvider "github.com/konveyor/asset-generation/pkg/providers/cf"
	korifiProvider "github.com/konveyor/asset-generation/pkg/providers/korifi"
	providerTypes "github.com/konveyor/asset-generation/pkg/providers/types"
)

func NewProvider(cfg Config) (Provider, error) {
	switch cfg.Type() {
	case providerTypes.ProviderTypeCF:
		cfCfg, ok := cfg.(*cfProvider.Config)
		if !ok {
			return nil, fmt.Errorf("invalid config type for cf")
		}
		return cf.New(cfCfg), nil
	case providerTypes.ProviderTypeKorifi:
		korifiCfg, ok := cfg.(*korifiProvider.Config)
		if !ok {
			return nil, fmt.Errorf("invalid config type for korifi")
		}
		return korifiProvider.New(korifiCfg), nil
	default:
		return nil, fmt.Errorf("unsupported provider type: %s", cfg.Type())
	}
}
