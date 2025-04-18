package api

import (
	"fmt"
	"regexp"
	"strings"

	kTModels "github.com/konveyor/asset-generation/pkg/discover/cloud_foundry/korifi/models"
)

func GetAppName(appEnv kTModels.AppEnvResponse) (string, error) {
	vcap, valid := appEnv.ApplicationEnvJSON["VCAP_APPLICATION"]
	if !valid {
		return "", fmt.Errorf("can't find ")
	}
	vcapMap, valid := vcap.(map[string]any)
	if !valid {
		return "", fmt.Errorf("VCAP_APPLICATION is not a valid map: %v", vcap)
	}

	appName, valid := vcapMap["application_name"]
	if !valid {
		return "", fmt.Errorf("can't find application name in VCAP_APPLICATION")
	}

	appNameStr, isString := appName.(string)
	if !isString {
		return "", fmt.Errorf("application_name is not a string: %v", appName)
	}

	return appNameStr, nil
}

// disallowedDNSCharactersRegex provides pattern for characters not allowed in a DNS Name
var disallowedDNSCharactersRegex = regexp.MustCompile(`[^a-z0-9\-]`)

// ReplaceStartingTerminatingHyphens replaces the first and last characters of a string if they are hyphens
func ReplaceStartingTerminatingHyphens(str, startReplaceStr, endReplaceStr string) string {
	first := str[0]
	last := str[len(str)-1]
	if first == '-' {
		fmt.Printf("Warning: The first character of the name %q are not alphanumeric.\n", str)
		str = startReplaceStr + str[1:]
	}
	if last == '-' {
		fmt.Printf("Warning: The last character of the name %q are not alphanumeric.", str)
		str = str[:len(str)-1] + endReplaceStr
	}
	return str
}

// NormalizeForMetadataName converts the string to be compatible for service name
func NormalizeForMetadataName(metadataName string) (string, error) {
	if metadataName == "" {
		return "", fmt.Errorf("failed to normalize for service/metadata name because it is an empty string")
	}
	newName := disallowedDNSCharactersRegex.ReplaceAllLiteralString(strings.ToLower(metadataName), "-")
	maxLength := 63
	if len(newName) > maxLength {
		newName = newName[0:maxLength]
	}
	newName = ReplaceStartingTerminatingHyphens(newName, "a", "z")
	if newName != metadataName {
		fmt.Printf("Changing metadata name from %s to %s\n", metadataName, newName)
	}
	return newName, nil
}

// ConvertMapToPointer converts a map[string]string to map[string]*string
func ConvertMapToPointer(input map[string]string) map[string]*string {
	output := make(map[string]*string)
	for key, value := range input {
		output[key] = &value
	}

	return output
}
