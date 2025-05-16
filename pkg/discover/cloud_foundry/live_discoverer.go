package cloud_foundry

import (
	"fmt"

	"github.com/go-logr/logr"
	cfProvider "github.com/konveyor/asset-generation/pkg/discover/cloud_foundry/cf/provider"
	kProvider "github.com/konveyor/asset-generation/pkg/discover/cloud_foundry/korifi/provider"
)

// type LiveDiscovererImpl[T any] struct {
// 	logger     *logr.Logger
// 	provider   Provider
// 	cfAPI      T
// 	spaceNames *[]string //Space name lists
// }

func NewLiveDiscoverer[T any](log logr.Logger, provider Provider, spaces *[]string) (*LiveDiscovererImpl[T], error) {
	if provider == nil {
		return nil, fmt.Errorf("provider is nil")
	}
	if provider.GetProviderType() == "korifi" {
		korifiProvider, ok := provider.(*kProvider.KorifiProvider)
		if !ok {
			return nil, fmt.Errorf("provider is not a Korifi provider")
		}
		client, err := provider.GetClient()
		if err != nil {
			return nil, fmt.Errorf("error creating Korifi client: %v", err)
		}

		korifiProvider.Discover()
		// return &LiveDiscovererImpl[T]{
		// 	cfAPI:      any(kApi.NewCFAPIClient(client.(*http.Client), korifiProvider.GetKorifiConfig().BaseURL)).(T),
		// 	logger:     &log,
		// 	provider:   provider,
		// 	spaceNames: spaces}, nil
	}

	if provider.GetProviderType() == "cf" {
		cfProvider, ok := provider.(*cfProvider.CFProvider)
		if !ok {
			return nil, fmt.Errorf("provider is not a CF provider")
		}
		client, err := cfProvider.GetClient()
		if err != nil {
			return nil, fmt.Errorf("error creating CloudFoundry client: %v", err)
		}
		cfProvider.Discover()
		// return &LiveDiscovererImpl[T]{
		// 	cfAPI:      any(kApi.NewCFAPIClient(client.(*client.Client), "testurl.com")).(T),
		// 	logger:     &log,
		// 	provider:   provider,
		// 	spaceNames: spaces}, nil
	}

	return nil, fmt.Errorf("unsupported provider type: %s", provider.GetProviderType())
}
