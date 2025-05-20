package providers

func Discover[T any](cfg Config) (T, error) {
	p, err := NewProvider(cfg)
	if err != nil {
		return nil, err
	}
	return p.Discover(cfg)
}
