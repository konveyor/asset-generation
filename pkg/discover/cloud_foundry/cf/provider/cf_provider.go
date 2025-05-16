package cf

import (
	"context"
	"fmt"
	"log"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/config"

	cfTypes "github.com/konveyor/asset-generation/pkg/discover/cloud_foundry"
)

type CFProvider struct {
	config   CFConfig
	cfClient *client.Client
}
type CFConfig struct {
	cfURL string
}

func NewCFProvider(cfconfig CFConfig) (*CFProvider, error) {

	cfg, err := config.NewFromCFHome()
	if err != nil {
		return nil, err
	}
	cf, err := client.New(cfg)
	if err != nil {
		return nil, err
	}
	fmt.Println("CF Client created successfully")
	return &CFProvider{
		config:   cfconfig,
		cfClient: cf}, nil
}

func (c *CFProvider) GetProviderType() cfTypes.ProviderType {
	return cfTypes.ProviderTypeCF
}

func (c *CFProvider) GetClient() (interface{}, error) {
	cfg, err := config.NewFromCFHome()
	if err != nil {
		return nil, err
	}
	cf, err := client.New(cfg)
	if err != nil {
		return nil, err
	}
	fmt.Println("CF Client created successfully")
	return cf, nil
}

func (c *CFProvider) Discover(spaceNames []string) error {
	if spaceNames == nil || len(spaceNames) == 0 {
		return fmt.Errorf("no spaces provided for discovery")
	}

	for _, spaceName := range spaceNames {
		ctx := context.Background()
		log.Println("Analyzing space: ", spaceName)
		spaceOpts := client.NewSpaceListOptions()
		spaceOpts.Names.EqualTo(spaceName)
		remoteSpace, err := c.cfClient.Spaces.First(ctx, spaceOpts)
		if err != nil {
			return fmt.Errorf("error finding CF space for spaceName %s: %v", spaceName, err)
		}
		appsOpt := client.NewAppListOptions()
		appsOpt.SpaceGUIDs.EqualTo(remoteSpace.GUID)
		apps, err := c.cfClient.Applications.ListAll(context.Background(), appsOpt)
		if err != nil {
			return fmt.Errorf("error listing CF apps for space %s: %v", spaceName, err)
		}
		log.Println("Apps discovered: ", len(apps))

		for _, app := range apps {
			log.Println("Processing app:", app.Name)
			// 	normalizedAppName, err := kApi.NormalizeForMetadataName(strings.TrimSpace(appName))
			// 	if err != nil {
			// 		return fmt.Errorf("error normalizing app name: %v", err)
			// 	}
			appEnv, err := c.cfClient.Applications.GetEnvironment(context.Background(), app.GUID)
			if err != nil {
				return fmt.Errorf("error getting environment for app %s: %v", app.GUID, err)
			}
			log.Println("App Environment Variables: ", appEnv)
			processOpts := client.NewProcessOptions()
			processOpts.SpaceGUIDs.EqualTo(remoteSpace.GUID)
			processes, err := c.cfClient.Processes.ListForAppAll(ctx, app.GUID, processOpts)
			if err != nil {
				return fmt.Errorf("error getting processes: %v", err)
			}

			appProcesses := cfTypes.AppManifestProcesses{}
			for _, proc := range processes {
				fmt.Println("Process: ", proc)
				procInstances := uint(proc.Instances)

				appProcesses = append(appProcesses, cfTypes.AppManifestProcess{
					Type:                         cfTypes.AppProcessType(proc.Type),
					Command:                      *proc.Command,
					DiskQuota:                    fmt.Sprintf("%d", proc.DiskInMB),
					HealthCheckType:              cfTypes.AppHealthCheckType(proc.HealthCheck.Type),
					HealthCheckHTTPEndpoint:      *proc.HealthCheck.Data.Endpoint,
					HealthCheckInvocationTimeout: uint(*proc.HealthCheck.Data.InvocationTimeout),
					Instances:                    &procInstances,
					LogRateLimitPerSecond:        fmt.Sprintf("%d", proc.LogRateLimitInBytesPerSecond),
					Memory:                       fmt.Sprintf("%dMB", proc.MemoryInMB),
					// Timeout
					ReadinessHealthCheckType:         cfTypes.AppHealthCheckType(proc.ReadinessCheck.Type),
					ReadinessHealthCheckHttpEndpoint: *proc.ReadinessCheck.Data.Endpoint,
					ReadinessHealthInvocationTimeout: uint(*proc.ReadinessCheck.Data.InvocationTimeout),
					ReadinessHealthCheckInterval:     uint(*proc.ReadinessCheck.Data.Interval),
					Lifecycle:                        string(app.Lifecycle.Type),
				})
			}
			routeOpts := client.NewRouteListOptions()
			routeOpts.SpaceGUIDs.EqualTo(remoteSpace.GUID)
			routes, err := c.cfClient.Routes.ListForAppAll(ctx, app.GUID, routeOpts)
			if err != nil {
				return fmt.Errorf("error getting processes: %v", err)
			}
			appRoutes := cfTypes.AppManifestRoutes{}
			for _, r := range routes {
				appRoutes = append(appRoutes, cfTypes.AppManifestRoute{
					Route:    r.URL,
					Protocol: cfTypes.AppRouteProtocol(r.Protocol),
					// TODO: Options: loadbalancing?
				})
			}

			// FIXME: uncomment this when we know how to handle them
			// allBuildpacks, err := c.cfClient.Buildpacks.ListAll(ctx, nil)
			// if err != nil {
			// 	return fmt.Errorf("error getting buildpacks: %v", err)
			// }

			// FIXME: uncomment this when we know how to handle them
			// stacks, err := c.cfClient.Stacks.ListAppsOnStackAll(ctx, nil)
			// if err != nil {
			// 	return fmt.Errorf("error getting buildpacks: %v", err)
			// }

			// FIXME: uncomment this when we know how to handle them
			// serviceOfferingOpts := client.NewServiceOfferingListOptions()
			// serviceOfferingOpts.SpaceGUIDs.EqualTo(remoteSpace.GUID)
			// serviceOfferings, err := c.cfClient.ServiceOfferings.ListAll(ctx, serviceOfferingOpts)
			// if err != nil {
			// 	return fmt.Errorf("error getting service offerings: %v", err)
			// }

			appManifest := cfTypes.AppManifest{
				Name: app.Name,
				Env:  appEnv.EnvVars, //TODO: Running, staging, appEnvVar, SystemEnvVar
				Metadata: &cfTypes.AppMetadata{
					Labels:      app.Metadata.Labels,
					Annotations: app.Metadata.Annotations,
				},
				Processes: &appProcesses,
				Routes:    &appRoutes,
				// AppManifestProcess
				// Buildpacks  //<--- There is the buildpack list but not per app
				// RandomRoute
				// NoRoute
				// Services
				// Sidecars
				// Stack
			}
			if cfTypes.WriteToYAMLFile(appManifest, fmt.Sprintf("manifest_%s_%s.yaml", spaceName, appManifest.Name)) != nil {
				return fmt.Errorf("error writing manifest to file: %v", err)
			}
		}
	}
	return nil
}
