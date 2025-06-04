package generate

type Provider interface {
	Generate() (map[string]string, error)
}
