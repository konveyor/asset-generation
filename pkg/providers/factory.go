package providers

import (
	"fmt"
	"io"
	"log"

	cfProvider "github.com/konveyor/asset-generation/pkg/providers/cloud_foundry"
)

func NewProvider(cfg Config, logger *log.Logger) (Provider, error) {
	if logger == nil {
		logger = log.New(io.Discard, "", log.LstdFlags) // No-op logger
	}
	switch t := cfg.(type) {
	case *cfProvider.Config:
		logger.Println("Creating new CF provider")
		return cfProvider.New(t, logger), nil
	default:
		return nil, fmt.Errorf("unsupported provider type: %s", t)
	}
}
