package provider

// DiscoverResult encapsulates the output of a discovery operation,
// separating regular application data from sensitive information.
type DiscoverResult struct {
	// Content contains the discovered application manifest and configuration data
	Content map[string]any
	// Secret contains sensitive information such as credentials and tokens
	Secret map[string]any
}
