package cf

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/config"

	"github.com/go-playground/validator/v10"
	helpers "github.com/konveyor/asset-generation/internal/helpers/yaml"
	cfTypes "github.com/konveyor/asset-generation/pkg/models"
	p "github.com/konveyor/asset-generation/pkg/providers/helpers"
	dTypes "github.com/konveyor/asset-generation/pkg/providers/types/discover"
	pTypes "github.com/konveyor/asset-generation/pkg/providers/types/provider"
	"gopkg.in/yaml.v3"
)

type Config struct {
	ManifestPath      string
	CFConfigPath      string // usato se esiste
	Username          string // usato se CFConfigPath è vuoto
	Password          string
	Token             string
	APIEndpoint       string
	SkipSslValidation bool
	SpaceNames        []string
	OutputFolder      string
}

type CFProvider struct {
	cfg *Config
}

func New[T any](cfg *Config) *CFProvider {
	return &CFProvider{cfg: cfg}
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
		log.Println("Manifest path provided, using it for local discovery")
		return c.discoverFromManifestFile()
	}

	// Live discovery
	if len(c.cfg.SpaceNames) == 0 {
		return nil, fmt.Errorf("no spaces provided for live discovery")
	}
	return c.discoverFromLiveAPI()
}

// DiscoverFromManifestFile reads the manifest file and returns a list of
// applications from the manifest.
// If the output folder is provided, it writes the manifest to a file in the
// output folder with the name "manifest_<app_name>.yaml".
// If the output folder is not provided, it returns a list of applications.
func (c *CFProvider) discoverFromManifestFile() ([]dTypes.Application, error) {
	data, err := os.ReadFile(c.cfg.ManifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest file: %w", err)
	}
	var manifest cfTypes.AppManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML: %w", err)
	}

	app, err := createApp(manifest)
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
		cfManifests, err := c.createCFManifest(spaceName)
		if err != nil {
			return nil, fmt.Errorf("error creating CF manifest for space '%s': %w", spaceName, err)
		}

		for _, m := range cfManifests {
			app, err := createApp(m)
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

func createApp(cfApp cfTypes.AppManifest) (dTypes.Application, error) {
	timeout := 60
	if cfApp.Timeout != 0 {
		timeout = int(cfApp.Timeout)
	}
	services, err := p.MarshalUnmarshal[dTypes.Services](cfApp.Services)
	if err != nil {
		return dTypes.Application{}, err
	}
	routeSpec := p.ParseRouteSpec(cfApp.Routes, cfApp.RandomRoute, cfApp.NoRoute)
	docker, err := p.MarshalUnmarshal[dTypes.Docker](cfApp.Docker)

	if err != nil {
		return dTypes.Application{}, err
	}
	sidecars, err := p.MarshalUnmarshal[dTypes.Sidecars](cfApp.Sidecars)
	if err != nil {
		return dTypes.Application{}, err
	}
	processes, err := p.MarshalUnmarshal[dTypes.Processes](cfApp.Processes)
	if err != nil {
		return dTypes.Application{}, err
	}
	var labels, annotations map[string]*string

	if cfApp.Metadata != nil {
		labels = cfApp.Metadata.Labels
		annotations = cfApp.Metadata.Annotations
	}
	appManifestProcess, inlineProcess, err := p.ParseProcessSpecs(cfApp)

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
		return dTypes.Application{}, errors.Join(validationErrors...)
	}
	return app, nil
}

func (c *CFProvider) createCFManifest(spaceName string) ([]cfTypes.AppManifest, error) {
	ctx := context.Background()
	log.Println("Analyzing space: ", spaceName)
	spaceOpts := client.NewSpaceListOptions()
	spaceOpts.Names.EqualTo(spaceName)
	cfClient, err := c.GetClient()
	if err != nil {
		return nil, fmt.Errorf("error creating CF client: %v", err)
	}
	remoteSpace, err := cfClient.Spaces.First(ctx, spaceOpts)
	if err != nil {
		return nil, fmt.Errorf("error finding CF space for spaceName %s: %v", spaceName, err)
	}
	appsOpt := client.NewAppListOptions()
	appsOpt.SpaceGUIDs.EqualTo(remoteSpace.GUID)
	apps, err := cfClient.Applications.ListAll(context.Background(), appsOpt)
	if err != nil {
		return nil, fmt.Errorf("error listing CF apps for space %s: %v", spaceName, err)
	}
	log.Println("Apps discovered: ", len(apps))

	appManifests := []cfTypes.AppManifest{}
	for _, app := range apps {
		log.Println("Processing app:", app.Name)
		appEnv, err := cfClient.Applications.GetEnvironment(context.Background(), app.GUID)
		if err != nil {
			return nil, fmt.Errorf("error getting environment for app %s: %v", app.GUID, err)
		}
		log.Printf("App Environment Variables: %s", appEnv)
		processOpts := client.NewProcessOptions()
		processOpts.SpaceGUIDs.EqualTo(remoteSpace.GUID)
		processes, err := cfClient.Processes.ListForAppAll(ctx, app.GUID, processOpts)
		if err != nil {
			return nil, fmt.Errorf("error getting processes: %v", err)
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
		routes, err := cfClient.Routes.ListForAppAll(ctx, app.GUID, routeOpts)
		if err != nil {
			return nil, fmt.Errorf("error getting processes: %v", err)
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
		if helpers.WriteToYAMLFile(appManifest, fmt.Sprintf("manifest_%s_%s.yaml", spaceName, appManifest.Name)) != nil {
			return nil, fmt.Errorf("error writing manifest to file: %v", err)
		}
		appManifests = append(appManifests, appManifest)
	}
	return appManifests, nil

}

type ValidationErrorList []error

func validateApplication(app dTypes.Application) ValidationErrorList {
	validate := validator.New(validator.WithRequiredStructEnabled())
	err := validate.Struct(app)
	if err != nil {
		var validationErrors ValidationErrorList
		for _, err := range err.(validator.ValidationErrors) {
			validationErrors = append(validationErrors,
				fmt.Errorf("field validation for key '%s' field '%s' failed on the '%s' tag", err.Namespace(), err.Field(), err.Tag()))
		}
		return validationErrors
	}
	return nil
}
