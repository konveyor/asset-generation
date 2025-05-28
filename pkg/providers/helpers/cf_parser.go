package helpers

import (
	"encoding/json"
	"errors"
	"fmt"

	cfTypes "github.com/konveyor/asset-generation/pkg/models"
	dTypes "github.com/konveyor/asset-generation/pkg/providers/types/discover"

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

func ParseProcessSpecs(cfApp cfTypes.AppManifest) (*dTypes.ProcessSpecTemplate, *dTypes.ProcessSpec, error) {
	var template dTypes.ProcessSpecTemplate
	var processSpec dTypes.ProcessSpec
	var err error
	// dafault values
	memory := "1G"
	instances := 1
	logRateLimit := "16K"

	if cfApp.Type == "" {
		// Handle template process
		template, err = MarshalUnmarshal[dTypes.ProcessSpecTemplate](cfApp)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to parse template spec: %w", err)
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

		if (template == dTypes.ProcessSpecTemplate{}) {
			return nil, nil, nil
		}
		return &template, nil, nil
	}
	// Handle inline process

	processSpec, err = MarshalUnmarshal[dTypes.ProcessSpec](cfApp)

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

func ParseRouteSpec(cfRoutes *cfTypes.AppManifestRoutes, randomRoute, noRoute bool) dTypes.RouteSpec {
	if noRoute {
		return dTypes.RouteSpec{
			NoRoute: noRoute,
		}
	}
	routeSpec := dTypes.RouteSpec{
		RandomRoute: randomRoute}
	if cfRoutes == nil {
		return routeSpec
	}
	routeSpec.Routes = ParseRoutes(*cfRoutes)
	return routeSpec
}

func ParseRoutes(cfRoutes cfTypes.AppManifestRoutes) dTypes.Routes {
	if cfRoutes == nil {
		return nil
	}
	routes := dTypes.Routes{}
	for _, cfRoute := range cfRoutes {
		options := dTypes.RouteOptions{}
		if cfRoute.Options != nil {
			options.LoadBalancing = dTypes.LoadBalancingType(cfRoute.Options.LoadBalancing)
		}
		r := dTypes.Route{
			Route:    cfRoute.Route,
			Protocol: dTypes.RouteProtocol(cfRoute.Protocol),
			Options:  options,
		}
		routes = append(routes, r)
	}

	return routes
}

func ParseHealthCheck(cfType cfTypes.AppHealthCheckType, cfEndpoint string, cfInterval, cfTimeout uint) dTypes.ProbeSpec {

	t := dTypes.PortProbeType
	if len(cfType) > 0 {
		t = dTypes.ProbeType(cfType)
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

	return dTypes.ProbeSpec{
		Type:     t,
		Endpoint: endpoint,
		Timeout:  timeout,
		Interval: interval,
	}
}

func ParseReadinessHealthCheck(cfType cfTypes.AppHealthCheckType, cfEndpoint string, cfInterval, cfTimeout uint) dTypes.ProbeSpec {
	t := dTypes.ProcessProbeType
	if len(cfType) > 0 {
		t = dTypes.ProbeType(cfType)
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

	return dTypes.ProbeSpec{
		Type:     t,
		Endpoint: endpoint,
		Timeout:  timeout,
		Interval: interval,
	}

}

func ParseCFApp(cfApp cfTypes.AppManifest) (dTypes.Application, error) {
	timeout := 60
	if cfApp.Timeout != 0 {
		timeout = int(cfApp.Timeout)
	}
	services, err := MarshalUnmarshal[dTypes.Services](cfApp.Services)
	if err != nil {
		return dTypes.Application{}, err
	}
	routeSpec := ParseRouteSpec(cfApp.Routes, cfApp.RandomRoute, cfApp.NoRoute)
	docker, err := MarshalUnmarshal[dTypes.Docker](cfApp.Docker)

	if err != nil {
		return dTypes.Application{}, err
	}
	sidecars, err := MarshalUnmarshal[dTypes.Sidecars](cfApp.Sidecars)
	if err != nil {
		return dTypes.Application{}, err
	}
	processes, err := MarshalUnmarshal[dTypes.Processes](cfApp.Processes)
	if err != nil {
		return dTypes.Application{}, err
	}
	var labels, annotations map[string]*string

	if cfApp.Metadata != nil {
		labels = cfApp.Metadata.Labels
		annotations = cfApp.Metadata.Annotations
	}
	appManifestProcess, inlineProcess, err := ParseProcessSpecs(cfApp)

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
