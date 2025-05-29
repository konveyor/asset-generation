package provider

type ProviderType string

const (
	ProviderTypeCloudFoundry ProviderType = "cloud foundry"
	ProviderTypeKorifi       ProviderType = "korifi"
)

type DiscoverResult struct {
	Content map[string]any
	Secret  map[string]any
}
