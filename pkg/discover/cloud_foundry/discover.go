package cloud_foundry

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/go-playground/validator/v10"
)

type ValidationErrorList []error

func Discover(cfApp AppManifest) (Application, error) {

	timeout := 60
	if cfApp.Timeout != 0 {
		timeout = int(cfApp.Timeout)
	}
	services, err := marshalUnmarshal[Services](cfApp.Services)
	if err != nil {
		return Application{}, err
	}
	routeSpec := parseRouteSpec(cfApp.Routes, cfApp.RandomRoute, cfApp.NoRoute)

	docker, err := marshalUnmarshal[Docker](cfApp.Docker)
	if err != nil {
		return Application{}, err
	}
	sidecars, err := marshalUnmarshal[Sidecars](cfApp.Sidecars)
	if err != nil {
		return Application{}, err
	}
	processes, err := marshalUnmarshal[Processes](cfApp.Processes)
	if err != nil {
		return Application{}, err
	}
	var labels, annotations map[string]*string

	if cfApp.Metadata != nil {
		labels = cfApp.Metadata.Labels
		annotations = cfApp.Metadata.Annotations
	}

	appManifestProcess, inlineProcess, err := parseProcessSpecs(cfApp)
	if err != nil {
		return Application{}, err
	}

	var appManifestProcessTemplate *ProcessSpecTemplate
	if appManifestProcess != nil {
		appManifestProcessTemplate = appManifestProcess
	}
	if inlineProcess != (nil) {
		processes = append(processes, *inlineProcess)
	}

	app := Application{
		Metadata: Metadata{
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
		return Application{}, errors.Join(validationErrors...)
	}

	return app, nil
}

func parseHealthCheck(cfType AppHealthCheckType, cfEndpoint string, cfInterval, cfTimeout uint) ProbeSpec {
	t := PortProbeType
	if len(cfType) > 0 {
		t = ProbeType(cfType)
	}
	endpoint := "/"
	if len(cfEndpoint) > 0 {
		endpoint = cfEndpoint
	}
	timeout := 1
	if cfTimeout != 0 {
		timeout = int(cfTimeout)
	}
	interval := 30
	if cfInterval > 0 {
		interval = int(cfInterval)
	}
	return ProbeSpec{
		Type:     t,
		Endpoint: endpoint,
		Timeout:  timeout,
		Interval: interval,
	}
}

func parseReadinessHealthCheck(cfType AppHealthCheckType, cfEndpoint string, cfInterval, cfTimeout uint) ProbeSpec {
	t := ProcessProbeType
	if len(cfType) > 0 {
		t = ProbeType(cfType)
	}
	endpoint := "/"
	if len(cfEndpoint) > 0 {
		endpoint = cfEndpoint
	}
	timeout := 1
	if cfTimeout != 0 {
		timeout = int(cfTimeout)
	}
	interval := 30
	if cfInterval > 0 {
		interval = int(cfInterval)
	}
	return ProbeSpec{
		Type:     t,
		Endpoint: endpoint,
		Timeout:  timeout,
		Interval: interval,
	}
}

// parseProcessSpecs creates a ProcessSpec if Type is defined inline or a
// ProcessSpecTemplate when Type is empty.
func parseProcessSpecs(cfApp AppManifest) (*ProcessSpecTemplate, *ProcessSpec, error) {
	var template ProcessSpecTemplate
	var processSpec ProcessSpec
	var err error

	// dafault values
	memory := "1G"
	instances := 1
	logRateLimit := "16K"

	if cfApp.Type == "" {
		// Handle template process
		template, err = marshalUnmarshal[ProcessSpecTemplate](cfApp)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to parse template spec: %w", err)
		}
		template.HealthCheck = parseHealthCheck(
			cfApp.HealthCheckType,
			cfApp.HealthCheckHTTPEndpoint,
			cfApp.HealthCheckInterval,
			cfApp.HealthCheckInvocationTimeout)
		template.ReadinessCheck = parseReadinessHealthCheck(
			cfApp.ReadinessHealthCheckType,
			cfApp.ReadinessHealthCheckHttpEndpoint,
			cfApp.ReadinessHealthCheckInterval,
			cfApp.ReadinessHealthInvocationTimeout)

		if cfApp.LogRateLimitPerSecond != "" {
			template.LogRateLimit = cfApp.LogRateLimitPerSecond
		}

		if (template == ProcessSpecTemplate{}) {
			return nil, nil, nil
		}

		return &template, nil, nil
	}

	// Handle inline process
	processSpec, err = marshalUnmarshal[ProcessSpec](cfApp)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse inline spec: %w", err)
	}

	if processSpec.Memory == "" {
		processSpec.Memory = memory
	}
	if processSpec.Instances == 0 {
		processSpec.Instances = instances
	}
	if cfApp.LogRateLimitPerSecond != "" {
		processSpec.LogRateLimit = cfApp.LogRateLimitPerSecond
	}
	if processSpec.LogRateLimit == "" {
		processSpec.LogRateLimit = logRateLimit
	}
	return nil, &processSpec, nil
}

// Generic helper for marshaling/unmarshaling between types
func marshalUnmarshal[T any](input interface{}) (T, error) {
	var result T
	b, err := json.Marshal(input)
	if err != nil {
		return result, err
	}
	err = json.Unmarshal(b, &result)
	return result, err
}

func parseRouteSpec(cfRoutes *AppManifestRoutes, randomRoute, noRoute bool) RouteSpec {
	if noRoute {
		return RouteSpec{
			NoRoute: noRoute,
		}
	}
	routeSpec := RouteSpec{
		RandomRoute: randomRoute,
	}

	if cfRoutes == nil {
		return routeSpec
	}

	routeSpec.Routes = parseRoutes(*cfRoutes)
	return routeSpec
}

func parseRoutes(cfRoutes AppManifestRoutes) Routes {
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

func validateApplication(app Application) ValidationErrorList {
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
