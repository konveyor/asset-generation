package helpers

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

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
