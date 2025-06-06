package cloud_foundry

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
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

type Config struct {
	ManifestPath string `json:"manifest_path" yaml:"manifest_path"`
	// CloudFoundryConfigPath string         `json:"cloud_foundry_config_path" yaml:"cloud_foundry_config_path"`
	// APIEndpoint            string         `json:"api_endpoint" yaml:"api_endpoint"`
	// SkipSSLValidation      bool           `json:"skip_ssl_validation" yaml:"skip_ssl_validation"`
	CloudFoundryConfig *config.Config `json:"cloud_foundry_config,omitempty" yaml:"cloud_foundry_config,omitempty"`
	SpaceNames         []string       `json:"space_names" yaml:"space_names"`
	// Cloud Foundry transient client
	Client *client.Client `json:"-" yaml:"-"`
}

type CloudFoundryProvider struct {
	cfg    *Config
	logger *log.Logger
	cli    *client.Client
}

// ClientProvider defines the interface for GetClient, only for testing.
type ClientProvider interface {
	GetClient() (*client.Client, error)
}

func New(cfg *Config, logger *log.Logger) (*CloudFoundryProvider, error) {
	var err error
	cp := CloudFoundryProvider{
		cfg:    cfg,
		logger: logger,
	}
	if cfg.CloudFoundryConfig != nil {
		cp.cli, err = cp.getClient()
		if err != nil {
			return nil, err
		}
	}
	return &cp, nil
}

func (c *CloudFoundryProvider) getClient() (*client.Client, error) {
	if c.cfg.Client != nil {
		return c.cfg.Client, nil
	}

	cf, err := client.New(c.cfg.CloudFoundryConfig)
	if err != nil {
		return nil, err
	}
	c.logger.Println("Cloud Foundry client created successfully")
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

type discoverInputParam struct {
	spaceName string
	appName   string
}

func (c *CloudFoundryProvider) Discover(RawData any) (*pTypes.DiscoverResult, error) {
	input, ok := RawData.(discoverInputParam)
	if !ok {
		return nil, fmt.Errorf("invalid type %s", reflect.TypeOf(RawData))
	}
	if c.cfg.ManifestPath != "" {
		return c.discoverFromManifest(input.appName)
	}
	return c.discoverFromLive(input.spaceName, input.appName)
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
				c.logger.Printf("error processing manifest file '%s': %v", filePath, err)
				continue
			}
			if appName == "" {
				c.logger.Printf("manifest file '%s' does not contain an app name", filePath)
				continue
			}
			c.logger.Printf("found app name '%s' in manifest file '%s'", appName, filePath)
			apps = append(apps, filePath)
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
		apps, err := c.listAppsBySpaceName(spaceName)
		if err != nil {
			return nil, fmt.Errorf("error listing Cloud Foundry apps for space %s: %v", spaceName, err)
		}
		c.logger.Printf("Apps discovered: %d\n", len(apps))

		appsInSpace := make([]discoverInputParam, 0, len(apps))
		for _, app := range apps {
			appsInSpace = append(appsInSpace, discoverInputParam{spaceName: spaceName, appName: app.Name})
		}
		appList[spaceName] = appsInSpace
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

func (c *CloudFoundryProvider) discoverFromManifest(filePath string) (*pTypes.DiscoverResult, error) {
	var discoverResult pTypes.DiscoverResult

	c.logger.Println("Manifest path provided, using it for local Cloud Foundry discover")

	d, err := c.discoverFromManifestFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("error discovering from Cloud Foundry manifest file: %v", err)
	}
	// Extract sensitive information and use UUID as references to the map[string]any structure that contains
	// the original values
	s := extractSensitiveInformation(d)
	discoverResult.Secret = s
	discoverResult.Content, err = StructToMap(d)
	if err != nil {
		return nil, fmt.Errorf("error converting discovered Cloud Foundry application to map: %v", err)
	}

	return &discoverResult, nil
}

func (c *CloudFoundryProvider) discoverFromLive(spaceName string, appName string) (*pTypes.DiscoverResult, error) {
	var discoverResult pTypes.DiscoverResult

	if appName == "" {
		return nil, fmt.Errorf("no app GUID provided for Cloud Foundry live discover")
	}
	if c.cfg.CloudFoundryConfig == nil {
		return nil, fmt.Errorf("missing required configuration: APIEndpoint and CloudFoundryConfigPath must be provided for Cloud Foundry live discover")
	}

	c.logger.Println("Starting live Cloud Foundry discovery for app with GUID:", appName)

	d, err := c.discoverFromLiveAPI(spaceName, appName)
	if err != nil {
		return nil, err
	}
	// Extract sensitive information and use UUID as references to the map[string]any structure that contains
	// the original values
	s := extractSensitiveInformation(d)
	discoverResult.Secret = s
	discoverResult.Content, err = StructToMap(d)
	if err != nil {
		return nil, err
	}

	return &discoverResult, nil
}

// discoverFromManifestFile reads a manifest file and returns a list of applications.
//
// If an output folder is specified:
//   - The manifest is written to a file named "manifest_<app_name>.yaml" in that folder.
//   - An empty application list is returned.
//
// If no output folder is specified:
//   - The function returns the list of applications parsed from the manifest.
func (c *CloudFoundryProvider) discoverFromManifestFile(filePath string) (*Application, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest file: %v", err)
	}
	var manifest cfTypes.AppManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML: %v", err)
	}

	app, err := parseCFApp(manifest)
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
	cfManifests, err := c.generateCFManifestFromLiveAPI(spaceName, appName)
	if err != nil {
		return nil, err
	}

	discoveredApp, err := parseCFApp(*cfManifests)
	if err != nil {
		return nil, err
	}

	return &discoveredApp, nil
}

func (c *CloudFoundryProvider) generateCFManifestFromLiveAPI(spaceName string, appName string) (*cfTypes.AppManifest, error) {
	ctx := context.Background()
	c.logger.Printf("Analyzing application %s", appName)

	// Retrieve app in space and app name
	app, err := c.getAppBySpaceAndAppName(spaceName, appName)
	if err != nil {
		return nil, err
	}

	c.logger.Println("Processing app:", app.Name)
	appEnv, err := c.cli.Applications.GetEnvironment(context.Background(), app.GUID)
	if err != nil {
		return nil, err
	}

	processes, err := c.cli.Processes.ListForAppAll(ctx, app.GUID, nil)
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
	routeOpts.SpaceGUIDs.EqualTo(app.Relationships.Space.Data.GUID)
	routes, err := c.cli.Routes.ListForAppAll(ctx, app.GUID, routeOpts)
	if err != nil {
		return nil, fmt.Errorf("error getting processes: %v", err)
	}
	appRoutes := cfTypes.AppManifestRoutes{}
	for _, r := range routes {
		destinations, err := c.cli.Routes.GetDestinations(ctx, r.GUID)
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

	// Sidecars
	sidecars, err := c.getSidecars(app.GUID)
	if err != nil {
		return nil, err
	}

	// Retrieve services required by the application
	appServices, err := getServicesFromApplicationEnvironment(appEnv.SystemEnvVars)
	if err != nil {
		return nil, fmt.Errorf("error getting services for app %s: %s", app.Name, err)
	}
	appManifest := cfTypes.AppManifest{
		Name: app.Name,
		Env:  appEnv.EnvVars, //TODO: Running, staging, appEnvVar
		Metadata: &cfTypes.AppMetadata{
			Labels:      app.Metadata.Labels,
			Annotations: app.Metadata.Annotations,
		},
		// AppManifestProcess
		Processes: &appProcesses,
		// AppRoutes
		Routes: &appRoutes,
		// Build Packs
		Buildpacks: app.Lifecycle.BuildpackData.Buildpacks,
		// RandomRoute cannot be determined at runtime
		// NoRoute
		NoRoute:  len(appRoutes) == 0,
		Services: &appServices,
		Sidecars: &sidecars,
		Stack:    app.Lifecycle.BuildpackData.Stack,
	}

	return &appManifest, nil

}

func (c *CloudFoundryProvider) getSidecars(appGUID string) (cfTypes.AppManifestSideCars, error) {
	list, err := c.cli.Sidecars.ListForAppAll(context.Background(), appGUID, nil)
	if err != nil {
		return nil, fmt.Errorf("error while retrieving sidecars for app %s: %s", appGUID, err)
	}
	sidecars := cfTypes.AppManifestSideCars{}
	for _, sc := range list {
		if sc.Origin != "user" {
			continue
		}
		pt := make([]cfTypes.AppProcessType, 0, len(sc.ProcessTypes))
		for _, t := range sc.ProcessTypes {
			p := cfTypes.AppProcessType(t)
			pt = append(pt, p)
		}
		sidecars = append(sidecars, cfTypes.AppManifestSideCar{
			Name:         sc.Name,
			ProcessTypes: pt,
			Command:      sc.Command,
			Memory:       sc.MemoryInMB,
		})
	}
	return sidecars, nil
}

// appVCAPServiceAttributes is a structure that maps the relevant JSON fields from a runtime service structure as
// defined in an application's VCAP_SERVICES environment variable.
// For more information follow this link:
// https://docs.cloudfoundry.org/devguide/deploy-apps/environment-variable.html#VCAP-SERVICES
type appVCAPServiceAttributes struct {
	Name        string          `json:"name"`
	Credentials json.RawMessage `json:"credentials,omitempty"`
}

func getServicesFromApplicationEnvironment(env map[string]json.RawMessage) (cfTypes.AppManifestServices, error) {
	appServices := cfTypes.AppManifestServices{}
	vcap, ok := env[vcapServices]
	if !ok {
		return appServices, nil
	}
	instanceServices := map[string]appVCAPServiceAttributes{}
	err := json.Unmarshal(vcap, &instanceServices)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal VCAP_SERVICES: %s", err)
	}
	for name, svc := range instanceServices {
		s := cfTypes.AppManifestService{Name: name, BindingName: svc.Name, Parameters: map[string]any{credentials: svc.Credentials}}
		appServices = append(appServices, s)
	}
	return appServices, nil
}

// func (c *CloudFoundryProvider) getStack(ctx context.Context, app *resource.App) (string, error) {
// 	var appStack string
// 	allStacks, err := c.cli.Stacks.ListAll(ctx, nil)
// 	if err != nil {
// 		return "", fmt.Errorf("error getting stacks: %v", err)
// 	}
// 	for _, stack := range allStacks {
// 		appsOnStack, err := c.cli.Stacks.ListAppsOnStackAll(ctx, stack.GUID, nil)
// 		if err != nil {
// 			return "", fmt.Errorf("error getting buildpacks: %v", err)
// 		}
// 		for _, appOnStack := range appsOnStack {
// 			if appOnStack.GUID == app.GUID {
// 				appStack = stack.Name
// 				break
// 			}
// 		}
// 		if appStack != "" {
// 			break
// 		}
// 	}
// 	return appStack, nil
// }

func (c *CloudFoundryProvider) getSpaceByName(spaceName string) (*resource.Space, error) {
	spaceOpts := client.NewSpaceListOptions()
	spaceOpts.Names.EqualTo(spaceName)
	remoteSpace, err := c.cli.Spaces.First(context.Background(), spaceOpts)
	if err != nil {
		return nil, fmt.Errorf("error finding Cloud Foundry space for name %s: %v", spaceName, err)
	}
	return remoteSpace, nil
}

func (c *CloudFoundryProvider) listAppsBySpaceName(spaceName string) ([]*resource.App, error) {
	space, err := c.getSpaceByName(spaceName)
	if err != nil {
		return nil, fmt.Errorf("error getting space for space name %s: %v", spaceName, err)
	}
	appsOpt := client.NewAppListOptions()
	appsOpt.SpaceGUIDs.EqualTo(space.GUID)
	apps, err := c.cli.Applications.ListAll(context.Background(), appsOpt)
	if err != nil {
		return nil, fmt.Errorf("error listing Cloud Foundry apps for space name %s: %v", space.Name, err)
	}
	return apps, nil
}

func (c *CloudFoundryProvider) getAppBySpaceAndAppName(spaceName string, appName string) (*resource.App, error) {
	space, err := c.getSpaceByName(spaceName)
	if err != nil {
		return nil, err
	}
	appsOpt := client.NewAppListOptions()
	appsOpt.Names.EqualTo(appName)
	appsOpt.SpaceGUIDs.EqualTo(space.GUID)
	app, err := c.cli.Applications.ListAll(context.Background(), appsOpt)
	if err != nil {
		return nil, fmt.Errorf("error listing Cloud Foundry apps: %s", err)
	}
	if len(app) == 0 {
		return nil, fmt.Errorf("no application found with GUID %s", appName)
	}
	if len(app) > 1 {
		return nil, fmt.Errorf("multiple applications found with GUID %s", appName)
	}
	return app[0], nil
}

func safePtr[T any](ptr *T, defaultVal T) T {
	if ptr != nil {
		return *ptr
	}
	return defaultVal
}
