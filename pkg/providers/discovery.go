package providers

import "log"

func Discover[T any](cfg Config, logger *log.Logger) (result T, err error) {
	p, err := NewProvider[T](cfg, logger)
	if err != nil {
		return result, err
	}
	return p.Discover()
}
