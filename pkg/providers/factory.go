package providers

import (
	"fmt"
	"log"

	cfProvider "github.com/konveyor/asset-generation/pkg/providers/cf"
	korifiProvider "github.com/konveyor/asset-generation/pkg/providers/korifi"
	providerTypes "github.com/konveyor/asset-generation/pkg/providers/types/provider"
)

func NewProvider[T any](cfg Config) (Provider[T], error) {
	switch cfg.Type() {
	case providerTypes.ProviderTypeCF:
		log.Println("Creating new CF provider")
		cfCfg, ok := cfg.(*cfProvider.Config)
		if !ok {
			return nil, fmt.Errorf("invalid config type for cf")
		}
		// cast esplicito per far combaciare i tipi
		return any(cfProvider.New[T](cfCfg)).(Provider[T]), nil
	case providerTypes.ProviderTypeKorifi:
		log.Println("Creating new Korifi provider")
		korifiCfg, ok := cfg.(*korifiProvider.Config)
		if !ok {
			return nil, fmt.Errorf("invalid config type for korifi")
		}
		return any(korifiProvider.New[T](korifiCfg)).(Provider[T]), nil
	default:
		return nil, fmt.Errorf("unsupported provider type: %s", cfg.Type())
	}
}
