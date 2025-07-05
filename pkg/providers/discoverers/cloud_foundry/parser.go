package cloud_foundry

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"

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

const (
	// dafault values for a process
	memory                = "1G"
	defaultInstanceNumber = 1
	logRateLimit          = "16K"
)

func processProcessProbes(cfApp cfTypes.AppManifestProcess) (ProbeSpec, ProbeSpec) {
	healthCheck := ParseHealthCheck(
		cfApp.HealthCheckType,
		cfApp.HealthCheckHTTPEndpoint,
		cfApp.HealthCheckInterval,
		cfApp.HealthCheckInvocationTimeout)
	readinessCheck := ParseReadinessHealthCheck(
		cfApp.ReadinessHealthCheckType,
		cfApp.ReadinessHealthCheckHttpEndpoint,
		cfApp.ReadinessHealthCheckInterval,
		cfApp.ReadinessHealthInvocationTimeout)
	return healthCheck, readinessCheck
}
func parseProcess(cfApp cfTypes.AppManifestProcess) (*ProcessSpec, error) {
	if string(cfApp.Type) != string(Web) && string(cfApp.Type) != string(Worker) {
		return nil, fmt.Errorf("unknown process type %s", cfApp.Type)
	}
	processSpec, err := MarshalUnmarshal[ProcessSpec](cfApp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse inline spec: %w", err)
	}

	processSpec.HealthCheck, processSpec.ReadinessCheck = processProcessProbes(cfApp)
	if processSpec.Memory == "" {
		processSpec.Memory = memory
	}
	if processSpec.Instances == 0 {
		processSpec.Instances = defaultInstanceNumber
	}
	if processSpec.LogRateLimit == "" {
		processSpec.LogRateLimit = logRateLimit
	}
	if cfApp.LogRateLimitPerSecond != "" {
		processSpec.LogRateLimit = cfApp.LogRateLimitPerSecond
	}
	if cfApp.DiskQuota != "" {
		processSpec.DiskQuota = cfApp.DiskQuota
	}
	return &processSpec, nil
}

func parseProcessTemplate(cfApp cfTypes.AppManifest) (*ProcessSpecTemplate, error) {
	// Handle template process
	template, err := MarshalUnmarshal[ProcessSpecTemplate](cfApp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template spec: %w", err)
	}
	if cfApp.Instances == nil {
		template = ProcessSpecTemplate{Instances: defaultInstanceNumber}
	}
	if cfApp.LogRateLimitPerSecond != "" {
		template.LogRateLimit = cfApp.LogRateLimitPerSecond
	}
	if cfApp.DiskQuota != "" {
		template.DiskQuota = cfApp.DiskQuota
	}
	return &template, nil
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

func parseSidecars(sidecars cfTypes.AppManifestSideCars) (Sidecars, error) {

	s := Sidecars{}
	for _, sc := range sidecars {
		t, err := parseSidecar(sc)
		if err != nil {
			return nil, err
		}
		s = append(s, *t)
	}
	return s, nil
}

func parseSidecarMemory(memInMB string) (int, error) {
	re := regexp.MustCompile(`[A-Za-z]+`)
	return strconv.Atoi(re.ReplaceAllString(memInMB, ""))

}

func parseSidecar(sidecar cfTypes.AppManifestSideCar) (*SidecarSpec, error) {
	var mem int
	var err error
	if len(sidecar.Memory) > 0 {
		mem, err = parseSidecarMemory(sidecar.Memory)
		if err != nil {
			return nil, fmt.Errorf("failed to parse memory value for sidecar %s: %s", sidecar.Name, err)
		}
	}
	s := SidecarSpec{
		Name:    sidecar.Name,
		Command: sidecar.Command,
		Memory:  mem,
	}
	for _, pt := range sidecar.ProcessTypes {
		p := ProcessType(pt)
		if p != Web && p != Worker {
			return nil, fmt.Errorf("unknown process type %s", pt)
		}
		s.ProcessTypes = append(s.ProcessTypes, p)
	}
	return &s, nil
}

func parseServices(services *cfTypes.AppManifestServices) (Services, error) {
	if services == nil {
		return nil, nil
	}
	var svcs Services
	for _, svc := range *services {
		s, err := parseService(svc)
		if err != nil {
			return nil, err
		}
		svcs = append(svcs, *s)
	}
	return svcs, nil

}

func parseService(service cfTypes.AppManifestService) (*ServiceSpec, error) {
	svc, err := MarshalUnmarshal[ServiceSpec](service)
	if err != nil {
		return nil, err
	}
	return &svc, nil
}

func parseProcesses(cfProcs *cfTypes.AppManifestProcesses) (Processes, error) {

	var procs Processes

	for _, proc := range *cfProcs {
		p, err := parseProcess(proc)
		if err != nil {
			return nil, err
		}
		procs = append(procs, *p)
	}
	return procs, nil
}

var parseCFApp = func(spaceName string, cfApp cfTypes.AppManifest) (Application, error) {
	timeout := 60
	if cfApp.Timeout != 0 && cfApp.Type == "" {
		timeout = int(cfApp.Timeout)
	}
	services, err := parseServices(cfApp.Services)
	if err != nil {
		return Application{}, err
	}
	routeSpec := ParseRouteSpec(cfApp.Routes, cfApp.RandomRoute, cfApp.NoRoute)
	docker, err := MarshalUnmarshal[Docker](cfApp.Docker)

	if err != nil {
		return Application{}, err
	}
	var sidecars Sidecars
	if cfApp.Sidecars != nil {
		sidecars, err = parseSidecars(*cfApp.Sidecars)
		if err != nil {
			return Application{}, err
		}
	}
	var processes Processes
	if cfApp.Processes != nil {
		processes, err = parseProcesses(cfApp.Processes)
		if err != nil {
			return Application{}, err
		}
	}
	var labels, annotations map[string]*string

	if cfApp.Metadata != nil {
		labels = cfApp.Metadata.Labels
		annotations = cfApp.Metadata.Annotations
	}

	app := Application{
		Metadata: Metadata{
			Name:        cfApp.Name,
			Space:       spaceName,
			Labels:      labels,
			Annotations: annotations,
		},
		Timeout:             timeout,
		BuildPacks:          cfApp.Buildpacks,
		Env:                 cfApp.Env,
		Stack:               cfApp.Stack,
		Services:            services,
		Routes:              routeSpec,
		Docker:              docker,
		Sidecars:            sidecars,
		Processes:           processes,
		Path:                cfApp.Path,
		ProcessSpecTemplate: &ProcessSpecTemplate{Instances: defaultInstanceNumber},
	}

	if cfApp.Type == "" {
		t, err := parseProcessTemplate(cfApp)
		if err != nil {
			return Application{}, err
		}
		app.ProcessSpecTemplate = t
	} else {
		inlineProcess, err := parseProcess(cfApp.AppManifestProcess)
		if err != nil {
			return Application{}, err
		}
		if inlineProcess != nil {
			app.Processes = append(app.Processes, *inlineProcess)
		}
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
			errorList = errors.Join(errorList, errors.New(detailedMsg))
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
