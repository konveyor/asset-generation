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
	vcapServices      string = "VCAP_SERVICES"
	credentials       string = "credentials"
	defaultLocalOrg   string = "local"
	defaultLocalSpace string = "local"
)

type Config struct {
	ManifestPath       string         `json:"manifest_path" yaml:"manifest_path"`
	CloudFoundryConfig *config.Config `json:"cloud_foundry_config,omitempty" yaml:"cloud_foundry_config,omitempty"`
	SpaceNames         []string       `json:"space_names" yaml:"space_names"`
	OrgNames           []string       `json:"org_names" yaml:"org_names"`
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

// New creates a new CloudFoundryProvider instance with the given configuration.
// If CloudFoundryConfig is provided, it initializes the Cloud Foundry client for live discovery.
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

// getClient initializes and returns a Cloud Foundry client.
// If a client already exists in the config, it returns that instance.
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

// ListApps retrieves a list of applications from Cloud Foundry.
// For live discovery: returns a map keyed by organization names, with values
// containing AppReference slices for all apps across all spaces in that org.
// For local manifests: returns a map keyed by "local" (as org name), with values
// containing AppReference slices for all apps across all spaces found in manifests.
//
// Example return structure:
//
//	map[string][]any{
//	  "org1": []any{AppReference{OrgName: "org1", SpaceName: "space1", AppName: "app1"}, ...},
//	  "org2": []any{AppReference{OrgName: "org2", SpaceName: "space2", AppName: "app2"}, ...},
//	}
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

// AppReference represents a discovered application with its organizational context.
type AppReference struct {
	OrgName   string `json:"orgName"`
	SpaceName string `json:"spaceName"`
	AppName   string `json:"appName"`
}

// Discover extracts detailed application information from the provided raw data.
// For live discovery, it queries the Cloud Foundry API. For local discovery, it reads manifest files.
func (c *CloudFoundryProvider) Discover(RawData any) (*pTypes.DiscoverResult, error) {
	input, ok := RawData.(AppReference)
	if !ok {
		return nil, fmt.Errorf("invalid type %s", reflect.TypeOf(RawData))
	}
	if c.cfg.ManifestPath != "" {
		return c.discoverFromManifest(input.AppName)
	}
	return c.discoverFromLive(input.OrgName, input.SpaceName, input.AppName)
}

// listAppsFromLocalManifests handles discovery of apps by reading local manifest files.
// Returns a map keyed by "local" (as org name) for consistency with live discovery.
func (c *CloudFoundryProvider) listAppsFromLocalManifests() (map[string][]any, error) {
	// Default to "local" org name for local discovery
	orgName := defaultLocalOrg

	c.logger.Info("Using manifest path for Cloud Foundry local discover", "manifest_path", c.cfg.ManifestPath)
	isDirResult, err := isDir(c.cfg.ManifestPath)
	if err != nil {
		return nil, fmt.Errorf("error checking if path is directory %s: %v", c.cfg.ManifestPath, err)
	}

	var apps []AppReference

	if isDirResult {
		files, err := os.ReadDir(c.cfg.ManifestPath)
		if err != nil {
			return nil, fmt.Errorf("error reading directory %s: %v", c.cfg.ManifestPath, err)
		}

		for _, file := range files {
			filePath := filepath.Join(c.cfg.ManifestPath, file.Name())

			appName, spaceName, err := c.getAppNameAndSpaceFromManifest(filePath)
			if err != nil {
				c.logger.Info("error processing manifest file", "file_path", filePath, "error", err)
				continue
			}
			if appName == "" {
				c.logger.Info("manifest file does not contain an app name", "file_path", filePath)
				continue
			}
			c.logger.Info("found app in manifest file", "app_name", appName, "space_name", spaceName, "file_path", filePath)
			apps = append(apps, AppReference{
				OrgName:   orgName,
				SpaceName: spaceName,
				AppName:   appName,
			})
		}
	} else {
		appName, spaceName, err := c.getAppNameAndSpaceFromManifest(c.cfg.ManifestPath)
		if err != nil {
			return nil, fmt.Errorf("error processing manifest file %s: %v", c.cfg.ManifestPath, err)
		}
		if appName == "" {
			return nil, fmt.Errorf("no app name found in manifest file %s", c.cfg.ManifestPath)
		}
		apps = append(apps, AppReference{
			OrgName:   orgName,
			SpaceName: spaceName,
			AppName:   appName,
		})
	}

	// Return all apps under "local" org for consistency with live discovery
	// Return empty map if no apps found
	if len(apps) == 0 {
		return map[string][]any{}, nil
	}
	return map[string][]any{orgName: toAnySlice(apps)}, nil
}

// getAppNameAndSpaceFromManifest extracts the app name and space name from a manifest file.
// Returns (appName, spaceName, error). SpaceName defaults to "local" if not specified in manifest.
func (c *CloudFoundryProvider) getAppNameAndSpaceFromManifest(filePath string) (string, string, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return "", "", fmt.Errorf("failed to stat file %q: %v", filePath, err)
	}
	if info.IsDir() {
		c.logger.Info("Skipping directory", "path", filePath)
		return "", "", nil
	}

	// Check file extension for YAML
	if !hasYAMLExtension(filePath) {
		c.logger.Info("Skipping non-YAML file", "path", filePath)
		return "", "", nil
	}

	c.logger.Info("Processing file.", "filename", filePath)

	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", "", fmt.Errorf("failed to read manifest file %q: %v", filePath, err)
	}

	var manifest cfTypes.AppManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		c.logger.Info("Failed to parse as single application manifest, will try Cloud Foundry manifest format", "file_path", filePath, "error", err)
	} else if manifest.Name != "" {
		c.logger.Info("Successfully parsed single application manifest", "file_path", filePath, "app_name", manifest.Name)
		return manifest.Name, defaultLocalSpace, nil
	}
	c.logger.Info("Single application manifest parsed but no app name found, trying Cloud Foundry manifest format", "file_path", filePath)

	var cfManifest cfTypes.CloudFoundryManifest
	if err := yaml.Unmarshal(data, &cfManifest); err != nil {
		return "", "", fmt.Errorf("failed to unmarshal YAML: %v", err)
	}
	if len(cfManifest.Applications) == 0 {
		return "", "", fmt.Errorf("no applications found in %s", filePath)
	}

	c.logger.Info("Successfully parsed Cloud Foundry manifest", "file_path", filePath, "application_count", len(cfManifest.Applications))

	app, err := parseCFApp(cfManifest.Space, *cfManifest.Applications[0])
	if err != nil {
		return "", "", err
	}

	if app.Name == "" {
		c.logger.Info("Cloud Foundry manifest parsed but application has no name", "file_path", filePath)
		return "", "", fmt.Errorf("no applications found in %s", filePath)
	}

	spaceName := cfManifest.Space
	if spaceName == "" {
		spaceName = defaultLocalSpace
	}

	c.logger.Info("Successfully extracted application name from Cloud Foundry manifest", "file_path", filePath, "app_name", app.Name, "space", spaceName)
	return app.Name, spaceName, nil
}

// listAppsFromCloudFoundry handles discovery of apps by querying the Cloud Foundry API.
// Returns a map keyed by organization name, with values containing all apps across all spaces in that org.
func (c *CloudFoundryProvider) listAppsFromCloudFoundry() (map[string][]any, error) {
	if len(c.cfg.OrgNames) == 0 {
		return nil, fmt.Errorf("at least one organization name must be specified")
	}

	appListByOrg := make(map[string][]any, len(c.cfg.OrgNames))

	// Get all organizations by their names
	orgs, err := c.getOrgsByNames(c.cfg.OrgNames)
	if err != nil {
		return nil, fmt.Errorf("error getting organizations: %v", err)
	}

	if len(orgs) == 0 {
		c.logger.Info("No organizations found matching the provided names", "org_names", c.cfg.OrgNames)
		return appListByOrg, nil
	}

	// Get all spaces filtered by org GUIDs and space names in a single API call
	spaces, err := c.getSpacesByNamesAndOrgs(c.cfg.SpaceNames, orgs)
	if err != nil {
		return nil, fmt.Errorf("error getting spaces: %v", err)
	}

	c.logger.Info("Discovered spaces", "count", len(spaces), "orgs", len(orgs))

	// Group spaces by organization for easier lookup
	spacesByOrgGUID := make(map[string][]*resource.Space)
	for _, space := range spaces {
		orgGUID := space.Relationships.Organization.Data.GUID
		spacesByOrgGUID[orgGUID] = append(spacesByOrgGUID[orgGUID], space)
	}

	// Process each organization
	for _, org := range orgs {
		c.logger.Info("Analyzing organization", "org", org.Name)

		orgSpaces := spacesByOrgGUID[org.GUID]

		// If specific spaces were requested, check which ones exist in this org
		if len(c.cfg.SpaceNames) > 0 {
			foundSpaceNames := make(map[string]bool)
			for _, space := range orgSpaces {
				foundSpaceNames[space.Name] = true
			}

			// Warn about missing spaces
			for _, requestedSpace := range c.cfg.SpaceNames {
				if !foundSpaceNames[requestedSpace] {
					c.logger.Info("Skipping space because it doesn't exist in this organization",
						"space", requestedSpace, "org", org.Name)
				}
			}
		}

		// Process apps in each space of this org
		for _, space := range orgSpaces {
			if err := c.processAppsInSpace(org, space, appListByOrg); err != nil {
				return nil, err
			}
		}
	}

	return appListByOrg, nil
}

// validateOrgAndSpace validates that the organization and space resources are properly initialized.
// Returns an error if any required fields are missing.
func validateOrgAndSpace(org *resource.Organization, space *resource.Space) error {
	if org == nil {
		return fmt.Errorf("organization cannot be nil")
	}
	if space == nil {
		return fmt.Errorf("space cannot be nil")
	}
	if org.GUID == "" {
		return fmt.Errorf("organization GUID cannot be empty")
	}
	if space.GUID == "" {
		return fmt.Errorf("space GUID cannot be empty")
	}
	return nil
}

// processAppsInSpace processes and adds apps from a space to the appListByOrg.
// The org and space are required. Apps are added to the list keyed by organization name.
func (c *CloudFoundryProvider) processAppsInSpace(org *resource.Organization, space *resource.Space, appListByOrg map[string][]any) error {
	if err := validateOrgAndSpace(org, space); err != nil {
		return err
	}

	apps, err := c.listAppsBySpace(space, org.GUID)
	if err != nil {
		return fmt.Errorf("error listing Cloud Foundry apps for space %s: %v", space.Name, err)
	}

	c.logger.Info("Apps discovered", "count", len(apps), "org_name", org.Name, "space_name", space.Name)

	for _, app := range apps {
		if app == nil {
			c.logger.Info("Skipping nil app reference")
			continue
		}
		appRef := AppReference{
			OrgName:   org.Name,
			SpaceName: space.Name,
			AppName:   app.Name,
		}
		appListByOrg[org.Name] = append(appListByOrg[org.Name], appRef)
	}

	return nil
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

// isLiveDiscover determines if the provider should perform live discovery from Cloud Foundry API.
// Returns true if no manifest path is specified, indicating live discovery mode.
func isLiveDiscover(cfg *Config) bool {
	return cfg.ManifestPath == ""
}

// isDir checks if the given path points to a directory.
func isDir(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	return info.IsDir(), nil
}

// hasYAMLExtension checks if the given filename has a YAML file extension (.yaml or .yml).
func hasYAMLExtension(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return ext == ".yaml" || ext == ".yml"
}

// discoverFromManifest discovers application information from local manifest files.
// It searches for a manifest file matching the given app name and extracts its details.
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

			name, spaceName, err := c.getAppNameAndSpaceFromManifest(filePath)
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
			c.logger.Info("found app in manifest file", "app_name", appName, "space_name", spaceName, "file_path", manifestFile)
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

// discoverFromLive discovers application information from the live Cloud Foundry API.
// It retrieves detailed configuration for the specified organization, space, and application.
func (c *CloudFoundryProvider) discoverFromLive(orgName string, spaceName string, appName string) (*pTypes.DiscoverResult, error) {
	var discoverResult pTypes.DiscoverResult

	if appName == "" {
		return nil, fmt.Errorf("no app GUID provided for Cloud Foundry live discover")
	}
	if c.cfg.CloudFoundryConfig == nil {
		return nil, fmt.Errorf("missing required configuration: APIEndpoint and CloudFoundryConfigPath must be provided for Cloud Foundry live discover")
	}

	c.logger.Info("Starting live Cloud Foundry discovery for app", "app_name", appName)

	d, err := c.discoverFromLiveAPI(orgName, spaceName, appName)
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
func (c *CloudFoundryProvider) discoverFromLiveAPI(orgName string, spaceName string, appName string) (*Application, error) {
	cfManifests, err := c.generateCFManifestFromLiveAPI(orgName, spaceName, appName)
	if err != nil {
		return nil, err
	}

	discoveredApp, err := parseCFApp(spaceName, *cfManifests)
	if err != nil {
		return nil, err
	}

	return &discoveredApp, nil
}

// getProcesses retrieves process information for the specified Cloud Foundry application.
// Returns process configurations including health checks, memory, and disk quotas.
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

// getRoutes retrieves route information for the specified Cloud Foundry application.
// Returns route configurations including URLs, protocols, and options.
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

// generateCFManifestFromLiveAPI generates a Cloud Foundry manifest by querying the live API.
// It retrieves complete application configuration including processes, routes, services, and sidecars.
func (c *CloudFoundryProvider) generateCFManifestFromLiveAPI(orgName string, spaceName string, appName string) (*cfTypes.AppManifest, error) {

	c.logger.Info("Analyzing application", "app_name", appName)

	// Retrieve app in space and app name
	app, err := c.getAppByOrgAndSpaceAndAppName(orgName, spaceName, appName)
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

// getDockerSpecification retrieves Docker configuration for the specified application.
// Returns Docker image information if the application uses Docker lifecycle, otherwise returns nil.
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

// getSidecars retrieves sidecar configurations for the specified Cloud Foundry application.
// Returns sidecar information including name, command, memory, and associated process types.
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

// getServicesFromApplicationEnvironment extracts service bindings from the application's VCAP_SERVICES environment variable.
// Returns service configurations including names, binding names, and credentials.
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

// getSpaceByNameInOrg retrieves a space by name within a specific organization.
// Returns an error if the space is not found or has invalid data.
func (c *CloudFoundryProvider) getSpaceByNameInOrg(spaceName string, orgGUID string) (*resource.Space, error) {
	spaceOpts := client.NewSpaceListOptions()
	spaceOpts.Names.EqualTo(spaceName)
	spaceOpts.OrganizationGUIDs.EqualTo(orgGUID)
	remoteSpace, err := c.cli.Spaces.First(context.Background(), spaceOpts)
	if err != nil {
		return nil, fmt.Errorf("error finding Cloud Foundry space for name '%s' in organization '%s': %v", spaceName, orgGUID, err)
	}
	if remoteSpace == nil {
		return nil, fmt.Errorf("Cloud Foundry API returned nil space for name '%s' in organization '%s'", spaceName, orgGUID)
	}
	if remoteSpace.GUID == "" {
		return nil, fmt.Errorf("Cloud Foundry space '%s' has empty GUID", spaceName)
	}
	return remoteSpace, nil
}

// getSpacesByNamesAndOrgs retrieves multiple spaces filtered by space names and organizations in a single API call.
// This leverages the CF API's ability to filter by multiple organization_guids and space names simultaneously.
func (c *CloudFoundryProvider) getSpacesByNamesAndOrgs(spaceNames []string, orgs []*resource.Organization) ([]*resource.Space, error) {
	if len(orgs) == 0 {
		return nil, fmt.Errorf("no organizations provided")
	}

	// Extract org GUIDs for the API call
	orgGUIDs := make([]string, 0, len(orgs))
	for _, org := range orgs {
		orgGUIDs = append(orgGUIDs, org.GUID)
	}

	spaceOpts := client.NewSpaceListOptions()
	// If spaceNames is empty, list all spaces in the orgs (no name filter)
	if len(spaceNames) > 0 {
		spaceOpts.Names.EqualTo(spaceNames...)
	} else {
		c.logger.Info("No space filter provided, listing all spaces in organizations", "org_count", len(orgs))
	}
	spaceOpts.OrganizationGUIDs.EqualTo(orgGUIDs...)
	spaces, err := c.cli.Spaces.ListAll(context.Background(), spaceOpts)
	if err != nil {
		return nil, fmt.Errorf("error listing Cloud Foundry spaces: %v", err)
	}

	return spaces, nil
}

// getOrgByName retrieves an organization by name from Cloud Foundry.
// Returns an error if the organization is not found or has invalid data.
func (c *CloudFoundryProvider) getOrgByName(orgName string) (*resource.Organization, error) {
	orgOpts := client.NewOrganizationListOptions()
	orgOpts.Names.EqualTo(orgName)
	remoteOrg, err := c.cli.Organizations.First(context.Background(), orgOpts)
	if err != nil {
		return nil, fmt.Errorf("error finding Cloud Foundry organization for name '%s': %v", orgName, err)
	}
	if remoteOrg == nil {
		return nil, fmt.Errorf("Cloud Foundry API returned nil organization for name '%s'", orgName)
	}
	if remoteOrg.GUID == "" {
		return nil, fmt.Errorf("Cloud Foundry organization '%s' has empty GUID", orgName)
	}
	return remoteOrg, nil
}

// getOrgsByNames retrieves multiple organizations by their names in a single API call.
func (c *CloudFoundryProvider) getOrgsByNames(orgNames []string) ([]*resource.Organization, error) {
	orgOpts := client.NewOrganizationListOptions()
	// If orgNames is empty, list all organizations (no name filter)
	if len(orgNames) > 0 {
		orgOpts.Names.EqualTo(orgNames...)
	}
	orgs, err := c.cli.Organizations.ListAll(context.Background(), orgOpts)
	if err != nil {
		return nil, fmt.Errorf("error listing Cloud Foundry organizations: %v", err)
	}

	return orgs, nil
}

// listAppsBySpace retrieves all applications within a specified space.
// Requires both the space resource and organization GUID.
func (c *CloudFoundryProvider) listAppsBySpace(space *resource.Space, orgID string) ([]*resource.App, error) {
	if space == nil {
		return nil, fmt.Errorf("space cannot be nil")
	}
	if space.GUID == "" {
		return nil, fmt.Errorf("space GUID cannot be empty for space: %s", space.Name)
	}
	if orgID == "" {
		return nil, fmt.Errorf("organization GUID cannot be empty")
	}

	appsOpt := client.NewAppListOptions()
	appsOpt.SpaceGUIDs.EqualTo(space.GUID)
	appsOpt.OrganizationGUIDs.EqualTo(orgID)
	apps, err := c.cli.Applications.ListAll(context.Background(), appsOpt)
	if err != nil {
		return nil, fmt.Errorf("error listing Cloud Foundry apps for space name %s: %v", space.Name, err)
	}
	return apps, nil
}

// getAppByOrgAndSpaceAndAppName retrieves a specific application by organization, space, and application name.
// Returns an error if multiple applications are found or if the application doesn't exist.
func (c *CloudFoundryProvider) getAppByOrgAndSpaceAndAppName(orgName string, spaceName string, appName string) (*resource.App, error) {
	org, err := c.getOrgByName(orgName)
	if err != nil {
		return nil, err
	}

	space, err := c.getSpaceByNameInOrg(spaceName, org.GUID)
	if err != nil {
		return nil, err
	}

	appsOpt := client.NewAppListOptions()
	appsOpt.Names.EqualTo(appName)
	appsOpt.SpaceGUIDs.EqualTo(space.GUID)
	appsOpt.OrganizationGUIDs.EqualTo(org.GUID)

	app, err := c.cli.Applications.ListAll(context.Background(), appsOpt)
	if err != nil {
		return nil, fmt.Errorf("error listing Cloud Foundry apps: %s", err)
	}
	if len(app) == 0 {
		return nil, fmt.Errorf("no application found with name %s in org %s and space %s", appName, orgName, spaceName)
	}
	if len(app) > 1 {
		return nil, fmt.Errorf("multiple applications found with name %s in org %s and space %s", appName, orgName, spaceName)
	}
	return app[0], nil
}

// safePtr safely dereferences a pointer and returns its value.
// If the pointer is nil, returns the provided default value.
func safePtr[T any](ptr *T, defaultVal T) T {
	if ptr != nil {
		return *ptr
	}
	return defaultVal
}

// toAnySlice converts a typed slice to a slice of any.
// This is useful for generic slice conversions required by interface definitions.
func toAnySlice[T any](input []T) []any {
	result := make([]any, len(input))
	for i, v := range input {
		result[i] = v
	}
	return result
}
