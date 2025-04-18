package cloud_foundry

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/go-logr/logr"
	kApi "github.com/konveyor/asset-generation/pkg/discover/cloud_foundry/korifi/api"
	kProvider "github.com/konveyor/asset-generation/pkg/discover/cloud_foundry/korifi/provider"
	"gopkg.in/yaml.v2"
)

type LiveDiscovererImpl struct {
	logger   *logr.Logger
	provider kProvider.KorifiProvider
	cfAPI    *kApi.CFAPIClient
	spaces   *[]string //GUID lists
}

func NewLiveDiscoverer(log logr.Logger, provider kProvider.KorifiProvider, spaces *[]string) (*LiveDiscovererImpl, error) {
	client, err := provider.GetKorifiHttpClient()
	if err != nil {
		return nil, fmt.Errorf("error creating Korifi client: %v", err)
	}
	return &LiveDiscovererImpl{
		cfAPI:    kApi.NewCFAPIClient(client, provider.GetKorifiConfig().BaseURL),
		logger:   &log,
		provider: provider,
		spaces:   spaces}, nil
}

func (ld *LiveDiscovererImpl) Discover() (*[]CloudFoundryManifest, error) {
	var spaces []string
	var manifests []CloudFoundryManifest
	if ld.spaces == nil || len(*ld.spaces) == 0 {
		cfSpaces, err := ld.cfAPI.ListSpaces()
		if err != nil {
			return nil, fmt.Errorf("error listing CF spaces: %v", err)
		}

		for _, space := range cfSpaces.Resources {
			spaces = append(spaces, space.GUID)
		}
	} else {
		spaces = *ld.spaces
	}

	for _, space := range spaces {
		log.Println("Analyzing space: ", space)

		apps, err := ld.cfAPI.ListApps(space)
		if err != nil {
			return nil, fmt.Errorf("error listing CF apps: %v", err)
		}

		log.Println("Apps discovered: ", apps.PaginationData.TotalResults)

		var cfManifest CloudFoundryManifest
		for _, app := range apps.Resources {
			log.Println("Processing app:", app.GUID)

			appEnv, err := ld.cfAPI.GetEnv(app.GUID)
			if err != nil {
				return nil, fmt.Errorf("error getting environment for app %s: %v", app.GUID, err)
			}

			appName, err := kApi.GetAppName(*appEnv)
			if err != nil {
				return nil, fmt.Errorf("error getting app name: %v", err)
			}

			normalizedAppName, err := kApi.NormalizeForMetadataName(strings.TrimSpace(appName))
			if err != nil {
				return nil, fmt.Errorf("error normalizing app name: %v", err)
			}

			process, err := ld.cfAPI.GetProcesses(app.GUID)
			if err != nil {
				return nil, fmt.Errorf("error getting processes: %v", err)
			}

			appProcesses := AppManifestProcesses{}
			for _, proc := range process.Resources {
				procInstances := uint(proc.Instances)

				appProcesses = append(appProcesses, AppManifestProcess{
					Type:                         AppProcessType(proc.Type),
					Command:                      proc.Command,
					DiskQuota:                    fmt.Sprintf("%d", proc.DiskQuotaMB),
					HealthCheckType:              AppHealthCheckType(proc.HealthCheck.Type),
					HealthCheckHTTPEndpoint:      proc.HealthCheck.Data.HTTPEndpoint,
					HealthCheckInvocationTimeout: uint(proc.HealthCheck.Data.InvocationTimeout),
					Instances:                    &procInstances,
					// LogRateLimitPerSecond
					Memory: fmt.Sprintf("%dMB", proc.MemoryMB),
					// Timeout
					// ReadinessHealthCheckType
					// ReadinessHealthCheckHttpEndpoint
					// ReadinessHealthInvocationTimeout
					// ReadinessHealthCheckInterval
					Lifecycle: string(app.Lifecycle.Type),
				})
			}

			routes, err := ld.cfAPI.GetRoutes(app.GUID)
			if err != nil {
				return nil, fmt.Errorf("error getting processes: %v", err)
			}
			appRoutes := AppManifestRoutes{}
			for _, r := range routes.Resources {
				appRoutes = append(appRoutes, AppManifestRoute{
					Route:    r.URL,
					Protocol: AppRouteProtocol(r.Protocol),
					// TODO: Options: loadbalancing?
				})
			}

			labels := kApi.ConvertMapToPointer(app.Metadata.Labels)
			annotations := kApi.ConvertMapToPointer(app.Metadata.Annotations)
			appManifest := AppManifest{
				Name: normalizedAppName,
				Env:  appEnv.EnvironmentVariables,
				Metadata: &AppMetadata{
					Labels:      labels,
					Annotations: annotations,
				},
				Processes: &appProcesses,
				Routes:    &appRoutes,
				// AppManifestProcess
				// Buildpacks
				// RandomRoute
				// NoRoute
				// Services
				// Sidecars
				// Stack
			}
			cfManifest.Applications = append(cfManifest.Applications, &appManifest)
		}

		err = writeToYAMLFile(cfManifest, "manifest.yaml")
		if err != nil {
			return nil, fmt.Errorf("error writing manifest to file: %v", err)
		}
		cfManifest.Space = space
		manifests = append(manifests, cfManifest)
	}
	return &manifests, nil
}

func writeToYAMLFile(data interface{}, filename string) error {
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
