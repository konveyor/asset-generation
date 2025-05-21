package providers

func Discover[T any](cfg Config) (result T, err error) {
	p, err := NewProvider[T](cfg)
	if err != nil {
		return result, err
	}
	return p.Discover()
}
