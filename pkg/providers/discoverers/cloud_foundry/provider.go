package cloud_foundry

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/config"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/google/uuid"

	"github.com/go-logr/logr"
	cfTypes "github.com/konveyor/asset-generation/internal/models"
	pTypes "github.com/konveyor/asset-generation/pkg/providers/types/provider"
	"gopkg.in/yaml.v3"
)

const (
	vcapServices string = "VCAP_SERVICES"
	credentials  string = "credentials"
)

type Config struct {
	ManifestPath       string         `json:"manifest_path" yaml:"manifest_path"`
	CloudFoundryConfig *config.Config `json:"cloud_foundry_config,omitempty" yaml:"cloud_foundry_config,omitempty"`
	SpaceNames         []string       `json:"space_names" yaml:"space_names"`
	// Cloud Foundry transient client
	Client *client.Client `json:"-" yaml:"-"`
}

type CloudFoundryProvider struct {
	cfg    *Config
	logger *logr.Logger
	cli    *client.Client
	// conceal extracts the sensitive information found in the CF manifest into a separate file and uses a
	// unique ID to link each of the items found between the discover manifest and this new file containing the
	// sensitive information
	conceal bool
}

// ClientProvider defines the interface for GetClient, only for testing.
type ClientProvider interface {
	GetClient() (*client.Client, error)
}

func New(cfg *Config, logger *logr.Logger, conceal bool) (*CloudFoundryProvider, error) {
	var err error
	cp := CloudFoundryProvider{
		cfg:     cfg,
		logger:  logger,
		conceal: conceal,
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
	c.logger.Info("Cloud Foundry client created successfully")
	c.cfg.Client = cf
	return cf, nil
}

// ListApps retrieves a list of application names from the specified Cloud
// Foundry space.
// It returns a map where the keys are space names and the values are slices of
// application names.
func (c *CloudFoundryProvider) ListApps() (map[string][]any, error) {
	if !isLiveDiscover(c.cfg) {
		apps, err := c.listAppsFromLocalManifests()
		if err != nil {
			return nil, err
		}
		return apps, nil
	}
	return c.listAppsFromCloudFoundry()
}

type AppReference struct {
	SpaceName string `json:"spaceName"`
	AppName   string `json:"appName"`
}

func (c *CloudFoundryProvider) Discover(RawData any) (*pTypes.DiscoverResult, error) {
	input, ok := RawData.(AppReference)
	if !ok {
		return nil, fmt.Errorf("invalid type %s", reflect.TypeOf(RawData))
	}
	if c.cfg.ManifestPath != "" {
		return c.discoverFromManifest(input.AppName)
	}
	return c.discoverFromLive(input.SpaceName, input.AppName)
}

// listAppsFromLocalManifests handles discovery of apps by reading local manifest files.
func (c *CloudFoundryProvider) listAppsFromLocalManifests() (map[string][]any, error) {
	c.logger.Info("Using manifest path for Cloud Foundry local discover", "manifest_path", c.cfg.ManifestPath)
	isDirResult, err := isDir(c.cfg.ManifestPath)
	if err != nil {
		return nil, fmt.Errorf("error checking if path is directory %s: %v", c.cfg.ManifestPath, err)
	}

	if isDirResult {
		files, err := os.ReadDir(c.cfg.ManifestPath)
		if err != nil {
			return nil, fmt.Errorf("error reading directory %s: %v", c.cfg.ManifestPath, err)
		}

		var apps []AppReference
		for _, file := range files {
			filePath := filepath.Join(c.cfg.ManifestPath, file.Name())

			appName, err := c.getAppNameFromManifest(filePath)
			if err != nil {
				c.logger.Info("error processing manifest file", "file_path", filePath, "error", err)
				continue
			}
			if appName == "" {
				c.logger.Info("manifest file does not contain an app name", "file_path", filePath)
				continue
			}
			c.logger.Info("found app name in manifest file", "app_name", appName, "file_path", filePath)
			apps = append(apps, AppReference{AppName: appName})
		}
		return map[string][]any{"local": toAnySlice(apps)}, nil
	} else {
		appName, err := c.getAppNameFromManifest(c.cfg.ManifestPath)
		if err != nil {
			return nil, fmt.Errorf("error processing manifest file %s: %v", c.cfg.ManifestPath, err)
		}
		if appName == "" {
			return nil, fmt.Errorf("no app name found in manifest file %s", c.cfg.ManifestPath)
		}
		return map[string][]any{"local": {AppReference{AppName: appName}}}, nil
	}
}

func (c *CloudFoundryProvider) getAppNameFromManifest(filePath string) (string, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to stat file %q: %v", filePath, err)
	}
	if info.IsDir() {
		c.logger.Info("Skipping directory", "path", filePath)
		return "", nil
	}

	// Check file extension for YAML
	if !hasYAMLExtension(filePath) {
		c.logger.Info("Skipping non-YAML file", "path", filePath)
		return "", nil
	}

	c.logger.Info("Processing file.", "filename", filePath)

	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read manifest file %q: %v", filePath, err)
	}

	var manifest cfTypes.AppManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return "", fmt.Errorf("failed to unmarshal YAML from %q: %v", filePath, err)
	}

	if manifest.Name == "" {
		c.logger.Info("Warning: manifest file does not contain a name", "file_path", filePath)
		return "", nil
	}

	return manifest.Name, nil
}

// listAppsFromCloudFoundry handles discovery of apps by querying the Cloud Foundry API.
func (c *CloudFoundryProvider) listAppsFromCloudFoundry() (map[string][]any, error) {
	appList := make(map[string][]any, len(c.cfg.SpaceNames))
	for _, spaceName := range c.cfg.SpaceNames {
		c.logger.Info("Analyzing space", "space_name", spaceName)
		apps, err := c.listAppsBySpaceName(spaceName)
		if err != nil {
			return nil, fmt.Errorf("error listing Cloud Foundry apps for space %s: %v", spaceName, err)
		}
		c.logger.Info("Apps discovered", "count", len(apps))

		appsInSpace := make([]AppReference, 0, len(apps))
		for _, app := range apps {
			appsInSpace = append(appsInSpace, AppReference{SpaceName: spaceName, AppName: app.Name})
		}
		appList[spaceName] = toAnySlice(appsInSpace)
	}
	return appList, nil
}

// extractSensitiveInformation captures the sensitive information (e.g. credentials) found in the service's credentials,
// including the environment values and the docker username if found,
// and stores it in a map[string]any structure to be appended to the discover structure returned to the caller using
// a UUID as reference.
// If the conceal flag is set to false no changes are made to the application manifest and the
// function returns an empty map
func (c CloudFoundryProvider) extractSensitiveInformation(app *Application) map[string]any {
	m := map[string]any{}
	if !c.conceal {
		return m
	}
	uuid.EnableRandPool() // Increases UUID generation speed by pregenerating a pool with this flag enabled
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

func (c *CloudFoundryProvider) discoverFromManifest(appName string) (*pTypes.DiscoverResult, error) {
	var discoverResult pTypes.DiscoverResult

	c.logger.Info("Manifest path provided, using it for local Cloud Foundry discover")

	isDirResult, err := isDir(c.cfg.ManifestPath)
	if err != nil {
		return nil, fmt.Errorf("error checking if path is directory %s: %v", c.cfg.ManifestPath, err)
	}
	var manifestFile string

	if isDirResult {
		files, err := os.ReadDir(c.cfg.ManifestPath)
		if err != nil {
			return nil, fmt.Errorf("error reading directory %s: %v", c.cfg.ManifestPath, err)
		}

		for _, file := range files {
			filePath := filepath.Join(c.cfg.ManifestPath, file.Name())

			name, err := c.getAppNameFromManifest(filePath)
			if err != nil {
				c.logger.Info("error processing manifest file", "file_path", filePath, "error", err)
				continue
			}
			if name == "" {
				c.logger.Info("manifest file does not contain an app name", "file_path", filePath)
				continue
			}
			if name != appName {
				continue
			}
			manifestFile = filePath
			c.logger.Info("found app name in manifest file", "app_name", appName, "file_path", manifestFile)
			break
		}
	} else {
		manifestFile = c.cfg.ManifestPath
	}

	d, err := c.discoverFromManifestFile(manifestFile)
	if err != nil {
		return nil, fmt.Errorf("error discovering from Cloud Foundry manifest file: %v", err)
	}
	// Extract sensitive information and use UUID as references to the map[string]any structure that contains
	// the original values
	s := c.extractSensitiveInformation(d)
	discoverResult.Secret = s
	discoverResult.Content, err = structToMap(d)
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

	c.logger.Info("Starting live Cloud Foundry discovery for app", "app_name", appName)

	d, err := c.discoverFromLiveAPI(spaceName, appName)
	if err != nil {
		return nil, err
	}
	// Extract sensitive information and use UUID as references to the map[string]any structure that contains
	// the original values
	s := c.extractSensitiveInformation(d)
	discoverResult.Secret = s
	discoverResult.Content, err = structToMap(d)
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

	if err = yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML: %v", err)
	}
	// Check if the file contains a single application that does not contain a space
	if !reflect.DeepEqual(manifest, cfTypes.AppManifest{}) {
		app, err := parseCFApp("", manifest)
		if err != nil {
			return nil, fmt.Errorf("failed to create application: %v", err)
		}
		return &app, nil
	}
	c.logger.Info("Failed to parse Application manifest. Will attempt again using the Cloud Foundry Application manifest", "error", err)
	var cfManifest cfTypes.CloudFoundryManifest
	if err := yaml.Unmarshal(data, &cfManifest); err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML: %v", err)
	}
	if len(cfManifest.Applications) == 0 {
		return nil, fmt.Errorf("no applications found in %s", filePath)
	}
	c.logger.Info("Found applications in manifest", "count", len(cfManifest.Applications), "file", filePath)
	app, err := parseCFApp(cfManifest.Space, *cfManifest.Applications[0])
	if err != nil {
		return nil, err
	}
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

	discoveredApp, err := parseCFApp(spaceName, *cfManifests)
	if err != nil {
		return nil, err
	}

	return &discoveredApp, nil
}

func (c *CloudFoundryProvider) getProcesses(appGUID, lifecycle string) (*cfTypes.AppManifestProcesses, error) {
	processes, err := c.cli.Processes.ListForAppAll(context.Background(), appGUID, nil)
	if err != nil {
		return nil, fmt.Errorf("error getting processes: %v", err)
	}

	if len(processes) == 0 {
		return nil, nil
	}
	appProcesses := cfTypes.AppManifestProcesses{}
	for _, proc := range processes {
		procInstances := uint(proc.Instances)
		resourceProcess, err := c.cli.Processes.Get(context.Background(), proc.GUID)
		if err != nil {
			return nil, fmt.Errorf("error getting process %s: %v", proc.GUID, err)
		}
		appProcesses = append(appProcesses, cfTypes.AppManifestProcess{
			Type:                             cfTypes.AppProcessType(proc.Type),
			Command:                          safePtr(resourceProcess.Command, ""),
			DiskQuota:                        strconv.Itoa(proc.DiskInMB),
			HealthCheckType:                  cfTypes.AppHealthCheckType(proc.HealthCheck.Type),
			HealthCheckHTTPEndpoint:          parseProbeEndpoint(proc.HealthCheck.Data.Endpoint, ProbeType(proc.HealthCheck.Type)),
			HealthCheckInvocationTimeout:     uint(parseProbeInvocationTimeout(proc.HealthCheck.Data.InvocationTimeout, ProbeType(proc.HealthCheck.Type))),
			HealthCheckInterval:              uint(parseProbeInterval(proc.HealthCheck.Data.Interval, ProbeType(proc.HealthCheck.Type))),
			Timeout:                          parseHealthCheckTimeout(proc.HealthCheck.Data.Timeout),
			Instances:                        &procInstances,
			LogRateLimitPerSecond:            strconv.Itoa(proc.LogRateLimitInBytesPerSecond),
			Memory:                           strconv.Itoa(proc.MemoryInMB),
			ReadinessHealthCheckType:         cfTypes.AppHealthCheckType(proc.ReadinessCheck.Type),
			ReadinessHealthCheckHttpEndpoint: parseProbeEndpoint(proc.ReadinessCheck.Data.Endpoint, ProbeType(proc.ReadinessCheck.Type)),
			ReadinessHealthInvocationTimeout: uint(parseProbeInvocationTimeout(proc.ReadinessCheck.Data.InvocationTimeout, ProbeType(proc.ReadinessCheck.Type))),
			ReadinessHealthCheckInterval:     uint(parseProbeInterval(proc.ReadinessCheck.Data.Interval, ProbeType(proc.ReadinessCheck.Type))),
			Lifecycle:                        lifecycle,
		})
	}
	return &appProcesses, nil
}

func (c *CloudFoundryProvider) getRoutes(appGUID string) (*cfTypes.AppManifestRoutes, error) {
	routeOpts := client.NewRouteListOptions()
	routes, err := c.cli.Routes.ListForAppAll(context.Background(), appGUID, routeOpts)
	if err != nil {
		return nil, fmt.Errorf("error getting processes: %v", err)
	}
	appRoutes := cfTypes.AppManifestRoutes{}
	for _, r := range routes {
		destinations, err := c.cli.Routes.GetDestinations(context.Background(), r.GUID)
		if err != nil {
			return nil, fmt.Errorf("error getting destinations for route %s: %v", r.GUID, err)
		}
		var protocol string
		if len(destinations.Destinations) > 0 {
			protocol = *destinations.Destinations[0].Protocol
		}
		var options *cfTypes.AppRouteOptions
		if r.Options != nil {
			options = &cfTypes.AppRouteOptions{LoadBalancing: r.Options.LoadBalancing}
		}
		appRoutes = append(appRoutes, cfTypes.AppManifestRoute{
			Route:    r.URL,
			Protocol: cfTypes.AppRouteProtocol(protocol),
			Options:  options,
		})
	}
	return &appRoutes, nil
}
func (c *CloudFoundryProvider) generateCFManifestFromLiveAPI(spaceName string, appName string) (*cfTypes.AppManifest, error) {

	c.logger.Info("Analyzing application", "app_name", appName)

	// Retrieve app in space and app name
	app, err := c.getAppBySpaceAndAppName(spaceName, appName)
	if err != nil {
		return nil, err
	}

	c.logger.Info("Processing app", "app_name", app.Name)
	appEnv, err := c.cli.Applications.GetEnvironment(context.Background(), app.GUID)
	if err != nil {
		return nil, err
	}

	appProcesses, err := c.getProcesses(app.GUID, string(app.Lifecycle.Type))

	if err != nil {
		return nil, err
	}
	appRoutes, err := c.getRoutes(app.GUID)
	if err != nil {
		return nil, err
	}

	c.cli.ServiceCredentialBindings.GetParameters(context.Background(), app.GUID)
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
	// Retrieve docker image pullspec when the buildpack is type docker
	dockerSpec, err := c.getDockerSpecification(*app)
	if err != nil {
		return nil, err
	}
	appManifest := cfTypes.AppManifest{
		Name:   app.Name,
		Env:    appEnv.EnvVars, //TODO: Running, staging, appEnvVar
		Docker: dockerSpec,
		Metadata: &cfTypes.AppMetadata{
			Labels:      app.Metadata.Labels,
			Annotations: app.Metadata.Annotations,
		},
		// AppManifestProcess
		Processes: appProcesses,
		// AppRoutes
		Routes: appRoutes,
		// Build Packs
		Buildpacks: app.Lifecycle.BuildpackData.Buildpacks,
		// RandomRoute cannot be determined at runtime
		// NoRoute
		NoRoute:  len(*appRoutes) == 0,
		Services: appServices,
		Sidecars: sidecars,
		Stack:    app.Lifecycle.BuildpackData.Stack,
	}

	return &appManifest, nil

}

func (c *CloudFoundryProvider) getDockerSpecification(app resource.App) (*cfTypes.AppManifestDocker, error) {

	docker := cfTypes.AppManifestDocker{}
	if app.Lifecycle.Type != "docker" {
		return nil, nil
	}
	d, err := c.cli.Droplets.GetCurrentForApp(context.Background(), app.GUID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve droplet for app %s", app.Name)
	}
	docker.Image = *d.Image
	return &docker, nil
}

func (c *CloudFoundryProvider) getSidecars(appGUID string) (*cfTypes.AppManifestSideCars, error) {
	list, err := c.cli.Sidecars.ListForAppAll(context.Background(), appGUID, nil)
	if err != nil {
		return nil, fmt.Errorf("error while retrieving sidecars for app %s: %s", appGUID, err)
	}
	if len(list) == 0 {
		return nil, nil
	}
	var sidecars cfTypes.AppManifestSideCars
	for _, sc := range list {
		pt := make([]cfTypes.AppProcessType, 0, len(sc.ProcessTypes))
		for _, t := range sc.ProcessTypes {
			p := cfTypes.AppProcessType(t)
			pt = append(pt, p)
		}
		sidecars = append(sidecars, cfTypes.AppManifestSideCar{
			Name:         sc.Name,
			ProcessTypes: pt,
			Command:      sc.Command,
			Memory:       strconv.Itoa(sc.MemoryInMB),
		})
	}
	return &sidecars, nil
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
	appServices := cfTypes.AppManifestServices{}
	vcap, ok := env[vcapServices]
	if !ok {
		return nil, nil
	}
	instanceServices := map[string]appVCAPServiceAttributes{}
	err := json.Unmarshal(vcap, &instanceServices)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal VCAP_SERVICES: %s", err)
	}
	for name, svc := range instanceServices {
		var creds map[string]any
		if len(svc.Credentials) > 0 {
			creds = map[string]any{}
			err := json.Unmarshal(svc.Credentials, &creds)
			if err != nil {
				return nil, err
			}
		}
		s := cfTypes.AppManifestService{Name: name, BindingName: svc.Name, Parameters: creds}
		appServices = append(appServices, s)
	}
	return &appServices, nil
}

func (c *CloudFoundryProvider) getSpaceByName(spaceName string) (*resource.Space, error) {
	spaceOpts := client.NewSpaceListOptions()
	spaceOpts.Names.EqualTo(spaceName)
	remoteSpace, err := c.cli.Spaces.First(context.Background(), spaceOpts)
	if err != nil {
		return nil, fmt.Errorf("error finding Cloud Foundry space for name '%s': %v", spaceName, err)
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

func toAnySlice[T any](input []T) []any {
	result := make([]any, len(input))
	for i, v := range input {
		result[i] = v
	}
	return result
}
