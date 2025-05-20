package helpers

import (
	"encoding/json"
	"fmt"

	cfTypes "github.com/konveyor/asset-generation/pkg/models"
	dTypes "github.com/konveyor/asset-generation/pkg/providers/types/discover"
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
	if dTypes.ProcessSpec.Memory == "" {
		processSpec.Memory = memory
	}
	if dTypes.ProcessSpec.Instances == 0 {
		processSpec.Instances = instances
	}
	if cfApp.LogRateLimitPerSecond != "" {
		processSpec.LogRateLimit = cfApp.LogRateLimitPerSecond
	}
	if dTypes.ProcessSpec.LogRateLimit == "" {
		processSpec.LogRateLimit = logRateLimit
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
