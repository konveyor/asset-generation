package generators

type Provider interface {
	Generate() (map[string]string, error)
}
