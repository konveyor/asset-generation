package cloud_foundry

import (
	"encoding/json"
	"errors"
	"fmt"

	cfTypes "github.com/konveyor/asset-generation/internal/models"

	"github.com/go-playground/validator/v10"
)

// Generic helper for marshaling/unmarshaling between types
func MarshalUnmarshal[T any](input interface{}) (T, error) {
	var result T

	b, err := json.Marshal(input)
	if err != nil {
		return result, err
	}
	err = json.Unmarshal(b, &result)
	return result, err
}

func ParseProcessSpecs(cfApp cfTypes.AppManifest) (*ProcessSpecTemplate, *ProcessSpec, error) {
	var template ProcessSpecTemplate
	var processSpec ProcessSpec
	var err error
	// dafault values
	memory := "1G"
	instances := 1
	logRateLimit := "16K"

	if cfApp.Type == "" {
		// Handle template process
		template, err = MarshalUnmarshal[ProcessSpecTemplate](cfApp)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to parse template spec: %w", err)
		}
		if (template == ProcessSpecTemplate{}) {
			return nil, nil, nil
		}

		template.HealthCheck = ParseHealthCheck(
			cfApp.HealthCheckType,
			cfApp.HealthCheckHTTPEndpoint,
			cfApp.HealthCheckInterval,
			cfApp.HealthCheckInvocationTimeout)
		template.ReadinessCheck = ParseReadinessHealthCheck(
			cfApp.ReadinessHealthCheckType,
			cfApp.ReadinessHealthCheckHttpEndpoint,
			cfApp.ReadinessHealthCheckInterval,
			cfApp.ReadinessHealthInvocationTimeout)

		if cfApp.LogRateLimitPerSecond != "" {
			template.LogRateLimit = cfApp.LogRateLimitPerSecond
		}

		return &template, nil, nil
	}
	// Handle inline process

	processSpec, err = MarshalUnmarshal[ProcessSpec](cfApp)

	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse inline spec: %w", err)
	}
	if processSpec.Memory == "" {
		processSpec.Memory = memory
	}
	if processSpec.Instances == 0 {
		processSpec.Instances = instances
	}
	if processSpec.LogRateLimit == "" {
		processSpec.LogRateLimit = logRateLimit
	}
	if cfApp.LogRateLimitPerSecond != "" {
		processSpec.LogRateLimit = cfApp.LogRateLimitPerSecond
	}
	return nil, &processSpec, nil

}

func ParseRouteSpec(cfRoutes *cfTypes.AppManifestRoutes, randomRoute, noRoute bool) RouteSpec {
	if noRoute {
		return RouteSpec{
			NoRoute: noRoute,
		}
	}
	routeSpec := RouteSpec{
		RandomRoute: randomRoute}
	if cfRoutes == nil {
		return routeSpec
	}
	routeSpec.Routes = ParseRoutes(*cfRoutes)
	return routeSpec
}

func ParseRoutes(cfRoutes cfTypes.AppManifestRoutes) Routes {
	if cfRoutes == nil {
		return nil
	}
	routes := Routes{}
	for _, cfRoute := range cfRoutes {
		options := RouteOptions{}
		if cfRoute.Options != nil {
			options.LoadBalancing = LoadBalancingType(cfRoute.Options.LoadBalancing)
		}
		r := Route{
			Route:    cfRoute.Route,
			Protocol: RouteProtocol(cfRoute.Protocol),
			Options:  options,
		}
		routes = append(routes, r)
	}

	return routes
}

// parseProbeType returns the probe type with a fallback to the provided default
func parseProbeType(cfType cfTypes.AppHealthCheckType, defaultType ProbeType) ProbeType {
	if len(cfType) > 0 {
		return ProbeType(cfType)
	}
	return defaultType
}

// parseProbeEndpoint returns the endpoint with a fallback to "/"
func parseProbeEndpoint(cfEndpoint *string) string {
	if cfEndpoint != nil && len(*cfEndpoint) > 0 {
		return *cfEndpoint
	}
	return "/"
}

// parseProbeTimeout handles both uint and int types and returns timeout with fallback to 1
func parseProbeTimeout[T uint | int](cfTimeout *T) int {
	if cfTimeout != nil && *cfTimeout != 0 {
		return int(*cfTimeout)
	}
	return 1
}

// parseProbeInterval handles both uint and int types and returns interval with fallback to 30
func parseProbeInterval[T uint | int](cfInterval *T) int {
	if cfInterval != nil && *cfInterval > 0 {
		return int(*cfInterval)
	}
	return 30
}

func ParseHealthCheck(cfType cfTypes.AppHealthCheckType, cfEndpoint string, cfInterval uint, cfTimeout uint) ProbeSpec {
	return ProbeSpec{
		Type:     parseProbeType(cfType, PortProbeType),
		Endpoint: parseProbeEndpoint(&cfEndpoint),
		Timeout:  parseProbeTimeout(&cfTimeout),
		Interval: parseProbeInterval(&cfInterval),
	}
}

func ParseReadinessHealthCheck(cfType cfTypes.AppHealthCheckType, cfEndpoint string, cfInterval uint, cfTimeout uint) ProbeSpec {
	return ProbeSpec{
		Type:     parseProbeType(cfType, ProcessProbeType),
		Endpoint: parseProbeEndpoint(&cfEndpoint),
		Timeout:  parseProbeTimeout(&cfTimeout),
		Interval: parseProbeInterval(&cfInterval),
	}
}

var parseCFApp = func(spaceName string, cfApp cfTypes.AppManifest) (Application, error) {
	timeout := 60
	if cfApp.Timeout != 0 {
		timeout = int(cfApp.Timeout)
	}
	services, err := MarshalUnmarshal[Services](cfApp.Services)
	if err != nil {
		return Application{}, err
	}
	routeSpec := ParseRouteSpec(cfApp.Routes, cfApp.RandomRoute, cfApp.NoRoute)
	docker, err := MarshalUnmarshal[Docker](cfApp.Docker)

	if err != nil {
		return Application{}, err
	}
	sidecars, err := MarshalUnmarshal[Sidecars](cfApp.Sidecars)
	if err != nil {
		return Application{}, err
	}
	processes, err := MarshalUnmarshal[Processes](cfApp.Processes)
	if err != nil {
		return Application{}, err
	}
	var labels, annotations map[string]*string

	if cfApp.Metadata != nil {
		labels = cfApp.Metadata.Labels
		annotations = cfApp.Metadata.Annotations
	}
	appManifestProcess, inlineProcess, err := ParseProcessSpecs(cfApp)

	if err != nil {
		return Application{}, err
	}
	var appManifestProcessTemplate *ProcessSpecTemplate

	if appManifestProcess != nil {
		appManifestProcessTemplate = appManifestProcess
	}
	if inlineProcess != nil {
		processes = append(processes, *inlineProcess)
	}
	app := Application{
		Metadata: Metadata{
			Name:        cfApp.Name,
			Space:       spaceName,
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
		return Application{}, validationErrors
	}
	return app, nil
}

func validateApplication(app Application) error {
	validate := validator.New(validator.WithRequiredStructEnabled())
	err := validate.Struct(app)
	if err != nil {
		var errorList error
		for _, err := range err.(validator.ValidationErrors) {
			detailedMsg := fmt.Sprintf(
				"\nvalidation failed for field '%s' (namespace: '%s'): actual value '%v' does not satisfy constraint '%s'",
				err.Field(),
				err.Namespace(),
				err.Value(),
				err.Tag(),
			)
			// Include parameter if available (e.g., max=10)
			if param := err.Param(); param != "" {
				detailedMsg += fmt.Sprintf("=%s", param)
			}
			errorList = errors.Join(errorList, fmt.Errorf(detailedMsg))
		}
		return errorList
	}
	return nil
}

func StructToMap(obj any) (map[string]any, error) {
	var m map[string]any
	b, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(b, &m)
	return m, err
}
