package providers

func LiveDiscover(cfg Config) ([]string, error) {
	p, err := NewProvider(cfg)
	if err != nil {
		return nil, err
	}
	return p.LiveDiscover()
}
