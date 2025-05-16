package cloud_foundry

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type ProviderType string

const (
	ProviderTypeCF     ProviderType = "cf"
	ProviderTypeKorifi ProviderType = "korifi"
)

type Provider interface {
	GetProviderType() ProviderType
	GetClient() (interface{}, error)
	Discover(spaceNames []string) error
}

func WriteToYAMLFile(data interface{}, filename string) error {
	yamlData, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Errorf("error marshaling to YAML: %w", err)
	}

	err = os.WriteFile(filename, yamlData, 0644)
	if err != nil {
		return fmt.Errorf("error writing YAML to file: %w", err)
	}

	return nil
}
