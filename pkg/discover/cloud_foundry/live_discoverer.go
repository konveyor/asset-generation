package cloud_foundry

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/go-logr/logr"
	korifi "github.com/konveyor/asset-generation/pkg/discover/cloud_foundry/korifi"
	korifiApi "github.com/konveyor/asset-generation/pkg/discover/cloud_foundry/korifi/api"
	"gopkg.in/yaml.v2"
)

type LiveDiscovererImpl struct {
	// client   *http.Client
	logger   *logr.Logger
	provider korifi.KorifiProvider
	cfAPI    *korifiApi.CFAPIClient
}

func NewLiveDiscoverer(log logr.Logger, provider korifi.KorifiProvider) (*LiveDiscovererImpl, error) {
	client, err := provider.GetKorifiHttpClient()
	if err != nil {
		return nil, fmt.Errorf("error creating Korifi client: %v", err)
	}
	return &LiveDiscovererImpl{cfAPI: korifiApi.NewCFAPIClient(client, "https://localhost/"), logger: &log, provider: provider}, nil
}

func (ld *LiveDiscovererImpl) Discover() (*CloudFoundryManifest, error) {
	apps, err := ld.cfAPI.ListApps()
	if err != nil {
		return nil, fmt.Errorf("error listing CF apps: %v", err)
	}

	log.Println("Apps discovered:", apps)

	var cfManifest CloudFoundryManifest
	for _, app := range apps.Resources {
		log.Println("Processing app:", app.GUID)

		appEnv, err := ld.cfAPI.GetEnv(app.GUID)
		if err != nil {
			return nil, fmt.Errorf("error getting environment for app %s: %v", app.GUID, err)
		}

		appName, err := korifiApi.GetAppName(*appEnv)
		if err != nil {
			return nil, fmt.Errorf("error getting app name: %v", err)
		}

		normalizedAppName, err := korifiApi.NormalizeForMetadataName(strings.TrimSpace(appName))
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

		labels := korifiApi.ConvertMapToPointer(app.Metadata.Labels)
		annotations := korifiApi.ConvertMapToPointer(app.Metadata.Annotations)
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

	return &cfManifest, nil
}

func writeToYAMLFile(data interface{}, filename string) error {
	// Marshal the data to YAML
	yamlData, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Errorf("error marshaling to YAML: %w", err)
	}

	// Write to file with 0644 permissions
	err = os.WriteFile(filename, yamlData, 0644)
	if err != nil {
		return fmt.Errorf("error writing YAML to file: %w", err)
	}

	return nil
}
