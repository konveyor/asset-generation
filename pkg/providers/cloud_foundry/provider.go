package cloud_foundry

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/config"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/go-playground/validator/v10"
	helpers "github.com/konveyor/asset-generation/internal/helpers/yaml"
	cfTypes "github.com/konveyor/asset-generation/pkg/models"
	pHelpers "github.com/konveyor/asset-generation/pkg/providers/helpers"
	dTypes "github.com/konveyor/asset-generation/pkg/providers/types/discover"
	pTypes "github.com/konveyor/asset-generation/pkg/providers/types/provider"
	"gopkg.in/yaml.v3"
)

type Config struct {
	ManifestPath      string
	CFConfigPath      string
	Username          string
	Password          string
	Token             string
	APIEndpoint       string
	SkipSSLValidation bool
	SpaceNames        []string
	OutputFolder      string
}

type CFProvider struct {
	cfg    *Config
	logger *log.Logger
}

func New[T any](cfg *Config, logger *log.Logger) *CFProvider {
	return &CFProvider{
		cfg:    cfg,
		logger: logger,
	}
}

func (cfg *Config) Type() pTypes.ProviderType {
	return pTypes.ProviderTypeCF
}

func (c *CFProvider) GetProviderType() pTypes.ProviderType {
	return pTypes.ProviderTypeCF
}

func (c *CFProvider) GetClient() (*client.Client, error) {
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

func (c *CFProvider) Discover() ([]dTypes.Application, error) {
	if c.cfg.ManifestPath != "" {
		c.logger.Println("Manifest path provided, using it for local discovery")
		return c.discoverFromManifestFile()
	}

	// Live discovery
	if len(c.cfg.SpaceNames) == 0 {
		return nil, fmt.Errorf("no spaces provided for live discovery")
	}

	if c.cfg.APIEndpoint == "" || c.cfg.Username == "" || (c.cfg.Password == "" && c.cfg.CFConfigPath == "") {
		return nil, fmt.Errorf("missing required configuration: APIEndpoint, Username, and either Password or CFConfigPath must be provided")
	}
	return c.discoverFromLiveAPI()
}

// discoverFromManifestFile reads a manifest file and returns a list of applications.
//
// If an output folder is specified:
//   - The manifest is written to a file named "manifest_<app_name>.yaml" in that folder.
//   - An empty application list is returned.
//
// If no output folder is specified:
//   - The function returns the list of applications parsed from the manifest.
func (c *CFProvider) discoverFromManifestFile() ([]dTypes.Application, error) {
	data, err := os.ReadFile(c.cfg.ManifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest file: %w", err)
	}
	var manifest cfTypes.AppManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML: %w", err)
	}

	app, err := parseCFApp(manifest)
	if err != nil {
		return nil, fmt.Errorf("failed to create application: %w", err)
	}
	if c.cfg.OutputFolder != "" {
		err = helpers.WriteToYAMLFile(manifest, fmt.Sprintf("%s/manifest_%s.yaml", c.cfg.OutputFolder, manifest.Name))
		if err != nil {
			return nil, fmt.Errorf("error writing manifest to file: %w", err)
		}
		return nil, nil
	}
	return []dTypes.Application{app}, nil
}

// discoverFromLiveAPI retrieves the application manifests from the live API
// and returns a list of applications.
// If the output folder is provided, it writes the manifest to a file in the
// output folder with the name "manifest_<space_name>_<app_name>.yaml".
// If the output folder is not provided, it returns a list of applications.
func (c *CFProvider) discoverFromLiveAPI() ([]dTypes.Application, error) {
	var apps []dTypes.Application
	writeToFile := c.cfg.OutputFolder != ""
	for _, spaceName := range c.cfg.SpaceNames {
		cfManifests, err := c.generateCFManifestFromLiveAPI(spaceName)
		if err != nil {
			return nil, fmt.Errorf("error creating CF manifest for space '%s': %w", spaceName, err)
		}

		for _, m := range cfManifests {
			app, err := parseCFApp(m)
			if err != nil {
				return nil, fmt.Errorf("failed to create app from manifest: %w", err)
			}
			if writeToFile {
				err = helpers.WriteToYAMLFile(m, fmt.Sprintf("%s/manifest_%s_%s.yaml", c.cfg.OutputFolder, spaceName, m.Name))
				if err != nil {
					return nil, fmt.Errorf("error writing manifest to file: %w", err)
				}
			} else {
				apps = append(apps, app)
			}
		}
	}
	return apps, nil
}

// func writeManifest(writer io.Writer, b []byte) error {
// 	ma := cfTypes.AppManifest{}
// 	err := yaml.Unmarshal(b, &ma)
// 	if err != nil {
// 		return err
// 	}
// 	a, err := Discover(ma)
// 	if err != nil {
// 		return err

// 	}
// 	fmt.Printf("discovered manifest: %v\n", a)
// 	b, err = yaml.Marshal(a)
// 	if err != nil {
// 		return err

// 	}
// 	if output == "" {
// 		fmt.Fprintf(writer, "%s", b)
// 		return nil
// 	}
// 	return os.WriteFile(output, b, 0644)
// }

func parseCFApp(cfApp cfTypes.AppManifest) (dTypes.Application, error) {
	timeout := 60
	if cfApp.Timeout != 0 {
		timeout = int(cfApp.Timeout)
	}
	services, err := pHelpers.MarshalUnmarshal[dTypes.Services](cfApp.Services)
	if err != nil {
		return dTypes.Application{}, err
	}
	routeSpec := pHelpers.ParseRouteSpec(cfApp.Routes, cfApp.RandomRoute, cfApp.NoRoute)
	docker, err := pHelpers.MarshalUnmarshal[dTypes.Docker](cfApp.Docker)

	if err != nil {
		return dTypes.Application{}, err
	}
	sidecars, err := pHelpers.MarshalUnmarshal[dTypes.Sidecars](cfApp.Sidecars)
	if err != nil {
		return dTypes.Application{}, err
	}
	processes, err := pHelpers.MarshalUnmarshal[dTypes.Processes](cfApp.Processes)
	if err != nil {
		return dTypes.Application{}, err
	}
	var labels, annotations map[string]*string

	if cfApp.Metadata != nil {
		labels = cfApp.Metadata.Labels
		annotations = cfApp.Metadata.Annotations
	}
	appManifestProcess, inlineProcess, err := pHelpers.ParseProcessSpecs(cfApp)

	if err != nil {
		return dTypes.Application{}, err
	}
	var appManifestProcessTemplate *dTypes.ProcessSpecTemplate

	if appManifestProcess != nil {
		appManifestProcessTemplate = appManifestProcess
	}
	if inlineProcess != (nil) {
		processes = append(processes, *inlineProcess)
	}
	app := dTypes.Application{
		Metadata: dTypes.Metadata{
			Name:        cfApp.Name,
			Labels:      labels,
			Annotations: annotations,
		},
		Timeout:    timeout,
		BuildPacks: cfApp.Buildpacks,
		Env:        cfApp.Env,
		Stack:      cfApp.Stack,
		Services:   services,
		Routes:     routeSpec,
		Docker:     docker,
		Sidecars:   sidecars,
		Processes:  processes,
	}
	if appManifestProcessTemplate != nil {
		app.ProcessSpecTemplate = *appManifestProcessTemplate
	}
	validationErrors := validateApplication(app)
	if validationErrors != nil {
		return dTypes.Application{}, validationErrors
	}
	return app, nil
}

func (c *CFProvider) generateCFManifestFromLiveAPI(spaceName string) ([]cfTypes.AppManifest, error) {
	ctx := context.Background()
	c.logger.Println("Analyzing space: ", spaceName)

	cfClient, err := c.GetClient()
	if err != nil {
		return nil, fmt.Errorf("error creating CF client: %v", err)
	}
	apps, err := listAppsBySpaceName(cfClient, spaceName)
	if err != nil {
		return nil, fmt.Errorf("error listing CF apps for space %s: %v", spaceName, err)
	}
	c.logger.Println("Apps discovered: ", len(apps))

	appManifests := []cfTypes.AppManifest{}
	for _, app := range apps {
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
		if err := helpers.WriteToYAMLFile(appManifest, fmt.Sprintf("manifest_%s_%s.yaml", spaceName, appManifest.Name)); err != nil {
			return nil, fmt.Errorf("error writing manifest to file: %v", err)
		}
		appManifests = append(appManifests, appManifest)
	}
	return appManifests, nil

}

func getSpaceGUIDByName(cfClient *client.Client, spaceName string) (string, error) {
	spaceOpts := client.NewSpaceListOptions()
	spaceOpts.Names.EqualTo(spaceName)
	remoteSpace, err := cfClient.Spaces.First(context.Background(), spaceOpts)
	if err != nil {
		return "", fmt.Errorf("error finding CF space for name %s: %v", spaceName, err)
	}
	return remoteSpace.GUID, nil
}
func listAppsBySpaceName(cfClient *client.Client, spaceGUID string) ([]*resource.App, error) {
	appsOpt := client.NewAppListOptions()
	appsOpt.SpaceGUIDs.EqualTo(spaceGUID)
	apps, err := cfClient.Applications.ListAll(context.Background(), appsOpt)
	if err != nil {
		return nil, fmt.Errorf("error listing CF apps for space %s: %v", spaceGUID, err)
	}
	return apps, nil
}

func validateApplication(app dTypes.Application) error {
	validate := validator.New(validator.WithRequiredStructEnabled())
	err := validate.Struct(app)
	if err != nil {
		var errorList error
		for _, err := range err.(validator.ValidationErrors) {
			msg2 := fmt.Sprintf(
				"\nvalidation failed for field '%s' (namespace: '%s'): actual value '%v' does not satisfy constraint '%s'",
				err.Field(),
				err.Namespace(),
				err.Value(),
				err.Tag(),
			)
			// Include parameter if available (e.g., max=10)
			if param := err.Param(); param != "" {
				msg2 += fmt.Sprintf("=%s", param)
			}
			errors.Join(errorList,
				fmt.Errorf(
					"field validation for key '%s' field '%s' failed on the '%s' tag",
					err.Namespace(),
					err.Field(),
					err.Tag()))

		}
		return errorList
	}
	return nil
}

func safePtr[T any](ptr *T, defaultVal T) T {
	if ptr != nil {
		return *ptr
	}
	return defaultVal
}
