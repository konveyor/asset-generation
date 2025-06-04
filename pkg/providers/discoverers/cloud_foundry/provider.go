package cloud_foundry

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/config"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/google/uuid"

	cfTypes "github.com/konveyor/asset-generation/pkg/models"
	pTypes "github.com/konveyor/asset-generation/pkg/providers/types/provider"
	"gopkg.in/yaml.v3"
)

const (
	vcapServices string = "VCAP_SERVICES"
	credentials  string = "credentials"
)

type AppDiscoveryConfig struct {
	SpaceName       string
	ApplicationName string
}

type Config struct {
	ManifestPath           string         `json:"manifest_path" yaml:"manifest_path"`
	CloudFoundryConfigPath string         `json:"cloud_foundry_config_path" yaml:"cloud_foundry_config_path"`
	APIEndpoint            string         `json:"api_endpoint" yaml:"api_endpoint"`
	SkipSSLValidation      bool           `json:"skip_ssl_validation" yaml:"skip_ssl_validation"`
	SpaceNames             []string       `json:"space_names" yaml:"space_names"`
	Client                 *client.Client `json:"-" yaml:"-"`
}

type CloudFoundryProvider struct {
	cfg    *Config
	logger *log.Logger
}

// ClientProvider defines the interface for GetClient, only for testing.
type ClientProvider interface {
	GetClient() (*client.Client, error)
}

func New(cfg *Config, logger *log.Logger) *CloudFoundryProvider {
	return &CloudFoundryProvider{
		cfg:    cfg,
		logger: logger,
	}
}

func (c *CloudFoundryProvider) GetClient() (*client.Client, error) {
	if c.cfg.Client != nil {
		return c.cfg.Client, nil
	}
	cfg, err := config.NewFromCFHome()
	if err != nil {
		return nil, err
	}
	cf, err := client.New(cfg)
	if err != nil {
		return nil, err
	}
	fmt.Println("Cloud Foundry client created successfully")
	c.cfg.Client = cf
	return cf, nil
}

// ListApps retrieves a list of application GUIDs from the specified Cloud
// Foundry space.
// It returns a slice of application GUIDs or an error in case of failure.
func (c *CloudFoundryProvider) ListApps() (map[string]any, error) {
	if !isLiveDiscover(c.cfg) {
		apps, err := c.listAppsFromLocalManifests()
		if err != nil {
			return nil, err
		}
		return map[string]any{"local": apps}, nil
	}
	return c.listAppsFromCloudFoundry()
}

func (c *CloudFoundryProvider) Discover(discoverConfig any) (pTypes.DiscoverResult, error) {

	dc, ok := discoverConfig.(AppDiscoveryConfig)
	if !ok {
		return pTypes.DiscoverResult{}, fmt.Errorf("invalid application discovery configuration type %s", reflect.TypeOf(discoverConfig))
	}
	if c.cfg.ManifestPath != "" {
		return c.discoverFromManifest()
	}
	return c.discoverFromLive(dc.SpaceName, dc.ApplicationName)
}

// listAppsFromLocalManifests handles discovery of apps by reading local manifest files.
func (c *CloudFoundryProvider) listAppsFromLocalManifests() (map[string]any, error) {
	c.logger.Println("Using manifest path for Cloud Foundry local discover:", c.cfg.ManifestPath)

	isDirResult, err := isDir(c.cfg.ManifestPath)
	if err != nil {
		return nil, fmt.Errorf("error checking if path is directory %s: %v", c.cfg.ManifestPath, err)
	}

	if isDirResult {
		files, err := os.ReadDir(c.cfg.ManifestPath)
		if err != nil {
			return nil, fmt.Errorf("error reading directory %s: %v", c.cfg.ManifestPath, err)
		}

		var apps []string
		for _, file := range files {
			filePath := filepath.Join(c.cfg.ManifestPath, file.Name())

			appName, err := c.getAppNameFromManifest(filePath)
			if err != nil {
				c.logger.Printf("error processing manifest file %s: %v", file.Name(), err)
				continue
			}
			if appName != "" {
				apps = append(apps, appName)
			}
		}
		return map[string]any{"local": apps}, nil
	} else {
		appName, err := c.getAppNameFromManifest(c.cfg.ManifestPath)
		if err != nil {
			return nil, fmt.Errorf("error processing manifest file %s: %v", c.cfg.ManifestPath, err)
		}
		if appName == "" {
			return nil, fmt.Errorf("no app name found in manifest file %s", c.cfg.ManifestPath)
		}
		return map[string]any{"local": appName}, nil
	}
}

func (c *CloudFoundryProvider) getAppNameFromManifest(filePath string) (string, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to stat file %q: %v", filePath, err)
	}
	if info.IsDir() {
		c.logger.Printf("Skipping directory: %s\n", filePath)
		return "", nil
	}

	// Check file extension for YAML
	if !hasYAMLExtension(filePath) {
		c.logger.Printf("Skipping non-YAML file: %s\n", filePath)
		return "", nil
	}

	c.logger.Printf("Processing file: %s\n", filePath)

	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read manifest file %q: %v", filePath, err)
	}

	var manifest cfTypes.AppManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return "", fmt.Errorf("failed to unmarshal YAML from %q: %v", filePath, err)
	}

	if manifest.Name == "" {
		c.logger.Printf("Warning: manifest file %q does not contain a name\n", filePath)
		return "", nil
	}

	return manifest.Name, nil
}

// listAppsFromCloudFoundry handles discovery of apps by querying the Cloud Foundry API.
func (c *CloudFoundryProvider) listAppsFromCloudFoundry() (map[string]any, error) {
	appList := make(map[string]any, len(c.cfg.SpaceNames))
	for _, spaceName := range c.cfg.SpaceNames {
		c.logger.Println("Analyzing space:", spaceName)

		cfClient, err := c.GetClient()
		if err != nil {
			return nil, fmt.Errorf("error creating Cloud Foundry client: %v", err)
		}

		apps, err := listAppsBySpaceName(cfClient, spaceName)
		if err != nil {
			return nil, fmt.Errorf("error listing Cloud Foundry apps for space %s: %v", spaceName, err)
		}

		c.logger.Printf("Apps discovered: %d\n", len(apps))

		appNames := make([]string, 0, len(apps))
		for _, app := range apps {
			appNames = append(appNames, app.Name)
		}
		appList[spaceName] = appNames
	}
	return appList, nil
}

// extractSensitiveInformation captures the sensitive information (e.g. credentials) found in the service's credentials,
// including the environment values and the docker username if found,
// and stores it in a map[string]any structure to be appended to the discover structure returned to the caller using
// a UUID as reference.
func extractSensitiveInformation(app *Application) map[string]any {
	uuid.EnableRandPool() // Increases UUID generation speed by pregenerating a pool with this flag enabled
	m := map[string]any{}
	if app.Docker.Username != "" {
		id := uuid.NewString()
		m[id] = app.Docker.Username
		app.Docker.Username = fmt.Sprintf("$(%s)", id)
	}
	for _, v := range app.Services {
		if c, ok := v.Parameters[credentials]; ok {
			id := uuid.NewString()
			m[id] = c
			v.Parameters[credentials] = fmt.Sprintf("$(%s)", id)
		}
	}
	return m
}

func isLiveDiscover(cfg *Config) bool {
	return cfg.ManifestPath == ""
}

func isDir(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	return info.IsDir(), nil
}
func hasYAMLExtension(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return ext == ".yaml" || ext == ".yml"
}

func (c *CloudFoundryProvider) discoverFromManifest() (pTypes.DiscoverResult, error) {
	var discoverResult pTypes.DiscoverResult

	c.logger.Println("Manifest path provided, using it for local Cloud Foundry discover")

	d, err := c.discoverFromManifestFile()
	if err != nil {
		return discoverResult, fmt.Errorf("error discovering from Cloud Foundry manifest file: %v", err)
	}
	// Extract sensitive information and use UUID as references to the map[string]any structure that contains
	// the original values
	s := extractSensitiveInformation(d)
	discoverResult.Secret = s
	discoverResult.Content, err = StructToMap(d)
	if err != nil {
		return discoverResult, fmt.Errorf("error converting discovered Cloud Foundry application to map: %v", err)
	}

	return discoverResult, nil
}

func (c *CloudFoundryProvider) discoverFromLive(spaceName string, appName string) (pTypes.DiscoverResult, error) {
	var discoverResult pTypes.DiscoverResult

	if spaceName == "" {
		return discoverResult, fmt.Errorf("no spaces provided for Cloud Foundry live discover")
	}
	if appName == "" {
		return discoverResult, fmt.Errorf("no app GUID provided for Cloud Foundry live discover")
	}
	if c.cfg.APIEndpoint == "" && c.cfg.CloudFoundryConfigPath == "" {
		return discoverResult, fmt.Errorf("missing required configuration: APIEndpoint and CloudFoundryConfigPath must be provided for Cloud Foundry live discover")
	}

	c.logger.Println("Starting live Cloud Foundry discovery for space:", spaceName)

	d, err := c.discoverFromLiveAPI(spaceName, appName)
	if err != nil {
		return discoverResult, fmt.Errorf("error during Cloud Foundry live discover: %v", err)
	}
	// Extract sensitive information and use UUID as references to the map[string]any structure that contains
	// the original values
	s := extractSensitiveInformation(d)
	discoverResult.Secret = s
	discoverResult.Content, err = StructToMap(d)
	if err != nil {
		return pTypes.DiscoverResult{}, fmt.Errorf("error converting discovered application to map: %s", err)
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
func (c *CloudFoundryProvider) discoverFromManifestFile() (*Application, error) {
	data, err := os.ReadFile(c.cfg.ManifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest file: %v", err)
	}
	var manifest cfTypes.AppManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML: %v", err)
	}

	app, err := ParseCFApp(manifest)
	if err != nil {
		return nil, fmt.Errorf("failed to create application: %v", err)
	}
	// extract sensitive information into a secret's structure
	return &app, nil
}

// discoverFromLiveAPI retrieves the application manifests from the live API
// and returns a list of applications.
// If the output folder is provided, it writes the manifest to a file in the
// output folder with the name "manifest_<space_name>_<app_name>.yaml".
// If the output folder is not provided, it returns a list of applications.
func (c *CloudFoundryProvider) discoverFromLiveAPI(spaceName string, appName string) (*Application, error) {
	space, err := getSpaceByName(c.cfg.Client, spaceName)
	if err != nil {
		return nil, fmt.Errorf("error getting space for space name %s", spaceName)
	}
	app, err := c.getAppByName(space.GUID, appName)
	if err != nil {
		return nil, fmt.Errorf("error while retrieving the application %s/%s", spaceName, appName)
	}
	cfManifests, err := c.generateCFManifestFromLiveAPI(space, app.GUID)
	if err != nil {
		return nil, fmt.Errorf("error creating Cloud Foundry manifest for space '%s': %v", spaceName, err)
	}

	discoveredApp, err := ParseCFApp(*cfManifests)
	if err != nil {
		return nil, fmt.Errorf("failed to create app from manifest: %v", err)
	}

	return &discoveredApp, nil
}

func (c *CloudFoundryProvider) getAppByName(spaceGUID string, appName string) (*resource.App, error) {
	appsOpt := client.NewAppListOptions()
	appsOpt.Names.EqualTo(appName)
	appsOpt.SpaceGUIDs.EqualTo(spaceGUID)
	return c.cfg.Client.Applications.First(context.Background(), appsOpt)
}

func (c *CloudFoundryProvider) generateCFManifestFromLiveAPI(space *resource.Space, appGUID string) (*cfTypes.AppManifest, error) {
	ctx := context.Background()
	c.logger.Println("Analyzing space: ", space.Name)

	cfClient, err := c.GetClient()
	if err != nil {
		return nil, fmt.Errorf("error creating Cloud Foundry client: %v", err)
	}
	app, err := getAppBySpaceName(cfClient, space.Name, appGUID)
	if err != nil {
		return nil, fmt.Errorf("error getting app by space name %s and app GUID %s: %v", space.Name, appGUID, err)
	}

	appManifest := cfTypes.AppManifest{}
	c.logger.Println("Processing app:", app.Name)
	appEnv, err := cfClient.Applications.GetEnvironment(context.Background(), app.GUID)
	if err != nil {
		return nil, fmt.Errorf("error getting environment for app %s: %v", app.GUID, err)
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
	routeOpts.SpaceGUIDs.EqualTo(space.GUID)
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
		return nil, fmt.Errorf("error listing all buildpacks: %s", err)
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
		return nil, fmt.Errorf("error getting stack for app %s: %s", app.Name, err)
	}

	// Retrieve services required by application
	appServices, err := getServicesFromApplicationEnvironment(appEnv.SystemEnvVars)
	if err != nil {
		return nil, fmt.Errorf("error getting services for app %s: %s", app.Name, err)
	}

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
		Services: appServices,
		// Sidecars
		Stack: appStack,
	}

	return &appManifest, nil

}

// appVCAPServiceAttributes is a structure that maps the relevant JSON fields from a runtime service structure as
// defined in an application's VCAP_SERVICES environment variable.
// For more information follow this link:
// https://docs.cloudfoundry.org/devguide/deploy-apps/environment-variable.html#VCAP-SERVICES
type appVCAPServiceAttributes struct {
	Name        string          `json:"name"`
	Credentials json.RawMessage `json:"credentials,omitempty"`
}

func getServicesFromApplicationEnvironment(env map[string]json.RawMessage) (*cfTypes.AppManifestServices, error) {
	vcap, ok := env[vcapServices]
	if !ok {
		return nil, fmt.Errorf("unable to find VCAP_SERVICES in Cloud Foundry environment")
	}
	appServices := cfTypes.AppManifestServices{}
	instanceServices := map[string]appVCAPServiceAttributes{}
	err := json.Unmarshal(vcap, &instanceServices)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal VCAP_SERVICES: %s", err)
	}
	for name, svc := range instanceServices {
		s := cfTypes.AppManifestService{Name: name, BindingName: svc.Name, Parameters: map[string]any{credentials: svc.Credentials}}
		appServices = append(appServices, s)
	}
	return &appServices, nil
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

func getSpaceByName(cfClient *client.Client, spaceName string) (*resource.Space, error) {
	spaceOpts := client.NewSpaceListOptions()
	spaceOpts.Names.EqualTo(spaceName)
	remoteSpace, err := cfClient.Spaces.First(context.Background(), spaceOpts)
	if err != nil {
		return nil, fmt.Errorf("error finding Cloud Foundry space for name %s: %v", spaceName, err)
	}
	return remoteSpace, nil
}

func listAppsBySpaceName(cfClient *client.Client, spaceName string) ([]*resource.App, error) {
	space, err := getSpaceByName(cfClient, spaceName)
	if err != nil {
		return nil, fmt.Errorf("error getting space for space name %s: %v", spaceName, err)
	}
	appsOpt := client.NewAppListOptions()
	appsOpt.SpaceGUIDs.EqualTo(space.GUID)
	apps, err := cfClient.Applications.ListAll(context.Background(), appsOpt)
	if err != nil {
		return nil, fmt.Errorf("error listing Cloud Foundry apps for space name %s: %v", space.Name, err)
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
