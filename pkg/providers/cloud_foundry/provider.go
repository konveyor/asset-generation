package cloud_foundry

import (
	"context"
	"fmt"
	"log"
	"os"
	"slices"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/config"
	"github.com/cloudfoundry/go-cfclient/v3/resource"

	cfTypes "github.com/konveyor/asset-generation/pkg/models"
	pHelpers "github.com/konveyor/asset-generation/pkg/providers/helpers"
	dTypes "github.com/konveyor/asset-generation/pkg/providers/types/discover"
	pTypes "github.com/konveyor/asset-generation/pkg/providers/types/provider"
	"gopkg.in/yaml.v3"
)

type Config struct {
	ManifestPath           string
	CloudFoundryConfigPath string
	Username               string
	Password               string
	Token                  string
	APIEndpoint            string
	SkipSSLValidation      bool
	SpaceName              string
	AppGUID                string
}

type CloudFoundryProvider struct {
	cfg    *Config
	logger *log.Logger
}

func New(cfg *Config, logger *log.Logger) *CloudFoundryProvider {
	return &CloudFoundryProvider{
		cfg:    cfg,
		logger: logger,
	}
}

func (c *CloudFoundryProvider) GetClient() (*client.Client, error) {
	cfg, err := config.NewFromCFHome()
	if err != nil {
		return nil, err
	}
	cf, err := client.New(cfg)
	if err != nil {
		return nil, err
	}
	fmt.Println("Cloud Foundry client created successfully")
	return cf, nil
}

// ListApps retrieves a list of application GUIDs from the specified Cloud
// Foundry space.
// It returns a slice of application GUIDs or an error in case of failure.
func (c *CloudFoundryProvider) ListApps() ([]string, error) {
	c.logger.Println("Analyzing space: ", c.cfg.SpaceName)

	cfClient, err := c.GetClient()
	if err != nil {
		return nil, fmt.Errorf("error creating Cloud Foundry client: %v", err)
	}
	apps, err := listAppsBySpaceName(cfClient, c.cfg.SpaceName)
	if err != nil {
		return nil, fmt.Errorf("error listing Cloud Foundry apps for space %s: %v", c.cfg.SpaceName, err)
	}
	c.logger.Println("Apps discovered: ", len(apps))

	appsGUIDs := make([]string, len(apps))
	for _, p := range apps {
		appsGUIDs = append(appsGUIDs, p.GUID)
	}
	return appsGUIDs, nil
}

func (c *CloudFoundryProvider) Discover() (pTypes.DiscoverResult, error) {
	var discoverResult pTypes.DiscoverResult
	if c.cfg.ManifestPath != "" {

		c.logger.Println("Manifest path provided, using it for local Cloud Foundry discover")
		d, err := c.discoverFromManifestFile()
		if err != nil {
			return discoverResult, fmt.Errorf("error discovering from Cloud Foundry manifest file: %w", err)
		}

		discoverResult.Content, err = pHelpers.StructToMap(d)
		if err != nil {
			return discoverResult, fmt.Errorf("error converting discovered Cloud Foundry application to map: %w", err)
		}
		return discoverResult, nil
	}

	// Live discover
	if c.cfg.SpaceName == "" {
		return discoverResult, fmt.Errorf("no spaces provided for Cloud Foundry live discover")
	}

	if c.cfg.AppGUID == "" {
		return discoverResult, fmt.Errorf("no app GUID provided for Cloud Foundry live discover")
	}

	if c.cfg.APIEndpoint == "" || c.cfg.Username == "" || (c.cfg.Password == "" && c.cfg.CloudFoundryConfigPath == "") {
		return discoverResult, fmt.Errorf("missing required configuration: APIEndpoint, Username, and either Password or CloudFoundryConfigPath must be provided for Cloud Foundry live discover")
	}

	d, err := c.discoverFromLiveAPI(c.cfg.SpaceName, c.cfg.AppGUID)
	if err != nil {
		return discoverResult, fmt.Errorf("error for Cloud Foundry live discover from manifest file: %w", err)
	}
	discoverResult.Content, err = pHelpers.StructToMap(d)
	if err != nil {
		return discoverResult, fmt.Errorf("error for for Cloud Foundry live discover converting discovered application to map: %w", err)
	}
	return discoverResult, nil
}

// discoverFromManifestFile reads a manifest file and returns a list of applications.
//
// If an output folder is specified:
//   - The manifest is written to a file named "manifest_<app_name>.yaml" in that folder.
//   - An empty application list is returned.
//
// If no output folder is specified:
//   - The function returns the list of applications parsed from the manifest.
func (c *CloudFoundryProvider) discoverFromManifestFile() (*dTypes.Application, error) {
	data, err := os.ReadFile(c.cfg.ManifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest file: %w", err)
	}
	var manifest cfTypes.AppManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML: %w", err)
	}

	app, err := pHelpers.ParseCFApp(manifest)
	if err != nil {
		return nil, fmt.Errorf("failed to create application: %w", err)
	}

	return &app, nil
}

// discoverFromLiveAPI retrieves the application manifests from the live API
// and returns a list of applications.
// If the output folder is provided, it writes the manifest to a file in the
// output folder with the name "manifest_<space_name>_<app_name>.yaml".
// If the output folder is not provided, it returns a list of applications.
func (c *CloudFoundryProvider) discoverFromLiveAPI(space string, appGUID string) (*dTypes.Application, error) {
	cfManifests, err := c.generateCFManifestFromLiveAPI(space, appGUID)
	if err != nil {
		return nil, fmt.Errorf("error creating Cloud Foundry manifest for space '%s': %w", space, err)
	}

	app, err := pHelpers.ParseCFApp(*cfManifests)
	if err != nil {
		return nil, fmt.Errorf("failed to create app from manifest: %w", err)
	}

	return &app, nil
}

func (c *CloudFoundryProvider) generateCFManifestFromLiveAPI(spaceName string, appGUID string) (*cfTypes.AppManifest, error) {
	ctx := context.Background()
	c.logger.Println("Analyzing space: ", spaceName)

	cfClient, err := c.GetClient()
	if err != nil {
		return nil, fmt.Errorf("error creating Cloud Foundry client: %v", err)
	}
	app, err := getAppBySpaceName(cfClient, spaceName, appGUID)
	if err != nil {
		return nil, fmt.Errorf("error getting app by space name %s and app GUID %s: %v", spaceName, appGUID, err)
	}

	appManifest := cfTypes.AppManifest{}
	c.logger.Println("Processing app:", app.Name)
	appEnv, err := cfClient.Applications.GetEnvironment(context.Background(), app.GUID)
	if err != nil {
		return nil, fmt.Errorf("error getting environment for app %s: %v", app.GUID, err)
	}
	spaceGUID, err := getSpaceGUIDByName(cfClient, spaceName)
	if err != nil {
		return nil, fmt.Errorf("error getting space GUID for space %s: %v", spaceName, err)
	}

	processes, err := cfClient.Processes.ListForAppAll(ctx, app.GUID, nil)
	if err != nil {
		return nil, fmt.Errorf("error getting processes: %v", err)
	}

	appProcesses := cfTypes.AppManifestProcesses{}
	for _, proc := range processes {
		fmt.Println("Process: ", proc)
		procInstances := uint(proc.Instances)

		appProcesses = append(appProcesses, cfTypes.AppManifestProcess{
			Type:                         cfTypes.AppProcessType(proc.Type),
			Command:                      safePtr(proc.Command, ""),
			DiskQuota:                    fmt.Sprintf("%d", proc.DiskInMB),
			HealthCheckType:              cfTypes.AppHealthCheckType(proc.HealthCheck.Type),
			HealthCheckHTTPEndpoint:      safePtr(proc.HealthCheck.Data.Endpoint, ""),
			HealthCheckInvocationTimeout: uint(safePtr(proc.HealthCheck.Data.InvocationTimeout, 0)),
			Instances:                    &procInstances,
			LogRateLimitPerSecond:        fmt.Sprintf("%d", proc.LogRateLimitInBytesPerSecond),
			Memory:                       fmt.Sprintf("%dMB", proc.MemoryInMB),
			// Timeout
			ReadinessHealthCheckType:         cfTypes.AppHealthCheckType(proc.ReadinessCheck.Type),
			ReadinessHealthCheckHttpEndpoint: safePtr(proc.ReadinessCheck.Data.Endpoint, ""),
			ReadinessHealthInvocationTimeout: uint(safePtr(proc.ReadinessCheck.Data.InvocationTimeout, 0)),
			ReadinessHealthCheckInterval:     uint(safePtr(proc.ReadinessCheck.Data.Interval, 0)),
			Lifecycle:                        string(app.Lifecycle.Type),
		})
	}
	routeOpts := client.NewRouteListOptions()
	routeOpts.SpaceGUIDs.EqualTo(spaceGUID)
	routes, err := cfClient.Routes.ListForAppAll(ctx, app.GUID, routeOpts)
	if err != nil {
		return nil, fmt.Errorf("error getting processes: %v", err)
	}
	appRoutes := cfTypes.AppManifestRoutes{}
	for _, r := range routes {
		destinations, err := cfClient.
			Routes.GetDestinations(ctx, r.GUID)
		if err != nil {
			return nil, fmt.Errorf("error getting destinations for route %s: %v", r.GUID, err)
		}
		var protocol string
		if len(destinations.Destinations) > 0 {
			protocol = *destinations.Destinations[0].Protocol
		}
		appRoutes = append(appRoutes, cfTypes.AppManifestRoute{
			Route:    r.URL,
			Protocol: cfTypes.AppRouteProtocol(protocol),
			// TODO: Options: loadbalancing?
		})

	}

	// There is the buildpack list but not per app
	allBuildpacks, err := cfClient.Buildpacks.ListAll(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("error getting buildpacks: %v", err)
	}
	appBuildpacks := []string{}
	if app.Lifecycle.Type == "buildpack" {
		// Filter buildpacks for the app by name
		for _, bp := range allBuildpacks {
			if slices.Contains(app.Lifecycle.BuildpackData.Buildpacks, bp.Name) {
				appBuildpacks = append(appBuildpacks, bp.Name)
			}
		}
	}

	// FIXME: uncomment this when we know how to handle them
	appStack, err := getStack(ctx, cfClient, app)
	if err != nil {
		return nil, fmt.Errorf("error getting stack for app %s: %v", app.Name, err)
	}

	// FIXME: uncomment this when we know how to handle them
	// serviceOfferingOpts := client.NewServiceOfferingListOptions()
	// serviceOfferingOpts.SpaceGUIDs.EqualTo(remoteSpace.GUID)
	// serviceOfferings, err := c.cfClient.ServiceOfferings.ListAll(ctx, serviceOfferingOpts)
	// if err != nil {
	// 	return fmt.Errorf("error getting service offerings: %v", err)
	// }

	appManifest = cfTypes.AppManifest{
		Name: app.Name,
		Env:  appEnv.EnvVars, //TODO: Running, staging, appEnvVar, SystemEnvVar
		Metadata: &cfTypes.AppMetadata{
			Labels:      app.Metadata.Labels,
			Annotations: app.Metadata.Annotations,
		},
		Processes: &appProcesses,
		Routes:    &appRoutes,
		// AppManifestProcess
		Buildpacks: appBuildpacks,
		// RandomRoute
		// NoRoute
		// Services
		// Sidecars
		Stack: appStack,
	}

	return &appManifest, nil

}

func getStack(ctx context.Context, cfClient *client.Client, app *resource.App) (string, error) {
	var appStack string
	allStacks, err := cfClient.Stacks.ListAll(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("error getting stacks: %v", err)
	}
	for _, stack := range allStacks {
		appsOnStack, err := cfClient.Stacks.ListAppsOnStackAll(ctx, stack.GUID, nil)
		if err != nil {
			return "", fmt.Errorf("error getting buildpacks: %v", err)
		}
		for _, appOnStack := range appsOnStack {
			if appOnStack.GUID == app.GUID {
				appStack = stack.Name
				break
			}
		}
		if appStack != "" {
			break
		}
	}
	return appStack, nil
}

func getSpaceGUIDByName(cfClient *client.Client, spaceName string) (string, error) {
	spaceOpts := client.NewSpaceListOptions()
	spaceOpts.Names.EqualTo(spaceName)
	remoteSpace, err := cfClient.Spaces.First(context.Background(), spaceOpts)
	if err != nil {
		return "", fmt.Errorf("error finding Cloud Foundry space for name %s: %v", spaceName, err)
	}
	return remoteSpace.GUID, nil
}

func listAppsBySpaceName(cfClient *client.Client, spaceGUID string) ([]*resource.App, error) {
	appsOpt := client.NewAppListOptions()
	appsOpt.SpaceGUIDs.EqualTo(spaceGUID)
	apps, err := cfClient.Applications.ListAll(context.Background(), appsOpt)
	if err != nil {
		return nil, fmt.Errorf("error listing Cloud Foundry apps for space %s: %v", spaceGUID, err)
	}
	return apps, nil
}

func getAppBySpaceName(cfClient *client.Client, spaceGUID string, appGUID string) (*resource.App, error) {
	appsOpt := client.NewAppListOptions()
	appsOpt.SpaceGUIDs.EqualTo(spaceGUID)
	appsOpt.GUIDs.EqualTo(appGUID)
	app, err := cfClient.Applications.ListAll(context.Background(), appsOpt)
	if err != nil {
		return nil, fmt.Errorf("error listing Cloud Foundry apps for space %s: %v", spaceGUID, err)
	}
	if len(app) > 1 {
		return nil, fmt.Errorf("multiple applications found for space %s with GUID %s", spaceGUID, appGUID)
	}
	return app[0], nil
}

func safePtr[T any](ptr *T, defaultVal T) T {
	if ptr != nil {
		return *ptr
	}
	return defaultVal
}
