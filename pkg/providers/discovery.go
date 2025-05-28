package providers

import "log"

func Discover[T any](cfg Config, logger *log.Logger, space string, appGUID string) (result T, err error) {
	p, err := NewProvider[T](cfg, logger)
	if err != nil {
		return result, err
	}
	return p.Discover(space, appGUID)
}

// func ListAppsBySpace[T any](cfg Config, logger *log.Logger) ([]string, error) {
// 	p, err := NewProvider[T](cfg, logger)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return p.ListAppsBySpace(cfg.)
// }
