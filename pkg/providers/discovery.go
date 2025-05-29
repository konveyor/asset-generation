package providers

import (
	"log"

	pTypes "github.com/konveyor/asset-generation/pkg/providers/types/provider"
)

func Discover(cfg Config, logger *log.Logger) (result pTypes.DiscoverResult, err error) {
	p, err := NewProvider(cfg, logger)
	if err != nil {
		return result, err
	}
	return p.Discover()
}
