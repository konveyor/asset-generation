package provider

type ProviderType string

const (
	ProviderTypeCloudFoundry ProviderType = "cloud foundry"
)

type DiscoverResult struct {
	Content map[string]any
	Secret  map[string]any
}
