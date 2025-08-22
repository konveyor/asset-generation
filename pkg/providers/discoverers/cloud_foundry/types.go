package cloud_foundry

// Application represents an interpretation of a runtime Cloud Foundry application. This structure differs in that
// the information it contains has been processed to simplify its transformation to a Kubernetes manifest using MTA
type Application struct {
	// Metadata captures the name, labels and annotations in the application.
	Metadata `yaml:",inline" json:",inline" validate:"required"`
	// Env captures the `env` field values in the CF application manifest.
	Env map[string]string `yaml:"env,omitempty" json:"env,omitempty"`
	// Routes represent the routes that are made available by the application.
	Routes RouteSpec `yaml:"routes,inline,omitempty" json:"routes,inline,omitempty" validate:"omitempty"`
	// Services captures the `services` field values in the CF application manifest.
	Services Services `yaml:"services,omitempty" json:"services,omitempty" validate:"omitempty,dive"`
	// Processes captures the `processes` field values in the CF application manifest.
	Processes Processes `yaml:"processes,omitempty" json:"processes,omitempty" validate:"omitempty,dive"`
	// Sidecars captures the `sidecars` field values in the CF application manifest.
	Sidecars Sidecars `yaml:"sidecars,omitempty" json:"sidecars,omitempty" validate:"omitempty,dive"`
	// Stack represents the `stack` field in the application manifest.
	// The value is captured for information purposes because it has no relevance
	// in Kubernetes.
	Stack string `yaml:"stack,omitempty" json:"stack,omitempty"`
	// BuildPacks capture the buildpacks defined in the CF application manifest.
	BuildPacks []string `yaml:"buildPacks,omitempty" json:"buildPacks,omitempty"`
	// Docker captures the Docker specification in the CF application manifest.
	Docker Docker `yaml:"docker,omitempty" json:"docker,omitempty" validate:"omitempty"`
	// ProcessSpec embeds the process specification details, which are inlined and validated if present.
	*ProcessSpecTemplate `yaml:",inline" json:",inline" validate:"omitempty"`
	// Path informs Cloud Foundry the locatino of the directory in which it can find your app.
	Path string `yaml:"path,omitempty" json:"path,omitempty" validate:"omitempty"`
	// Feature represents a map of key/value pairs of the app feature names to boolean values indicating whether the feature is enabled or not
	Features map[string]bool `yaml:"features,omitempty" json:"features,omitempty" validate:"omitempty"`
}

type Services []ServiceSpec
type Processes []ProcessSpec
type Sidecars []SidecarSpec

type Docker struct {
	// Image represents the pullspect where the container image is located.
	Image string `yaml:"image" json:"image" validate:"required"`
	// Username captures the username to authenticate against the container registry.
	Username string `yaml:"username,omitempty" json:"username,omitempty"`
}

type SidecarSpec struct {
	// Name represents the name of the Sidecar
	Name string `yaml:"name" json:"name" validate:"required"`
	// ProcessTypes captures the different process types defined for the sidecar.
	// Compared to a Process, which has only one type, sidecar processes can
	// accumulate more than one type.
	ProcessTypes []ProcessType `yaml:"processType" json:"processType" validate:"required"`
	// Command captures the command to run the sidecar
	Command string `yaml:"command" json:"command" validate:"required"`
	// Memory represents the amount of memory in MB to allocate to the sidecar.
	// Reference: https://v3-apidocs.cloudfoundry.org/version/3.192.0/index.html#the-sidecar-object
	// It's an optional field.
	// In the CF documentation it is referenced as an int when retrieving from a running application (live connection)
	// but it is defined as a string (e.g: '800MB') in a manifest file.
	Memory int `yaml:"memory,omitempty" json:"memory,omitempty"`
}

type ServiceSpec struct {
	// Name represents the name of the Cloud Foundry service required by the
	// application. This field represents the runtime name of the service, captured
	// from the 3 different cases where the service name can be listed.
	// For more information check https://docs.cloudfoundry.org/devguide/deploy-apps/manifest-attributes.html#services-block
	Name string `yaml:"name" json:"name" validate:"required"`
	// Parameters contain the k/v relationship for the aplication to bind to the service
	Parameters map[string]interface{} `yaml:"parameters,omitempty" json:"parameters,omitempty"`
	// BindingName captures the name of the service to bind to.
	BindingName string `yaml:"bindingName,omitempty" json:"bindingName,omitempty"`
}

type Metadata struct {
	// Name capture the `name` field int CF application manifest
	Name string `yaml:"name" json:"name" validate:"required"`
	// Space captures the `space` where the CF application is deployed at runtime. The field is empty if the
	// application is discovered directly from the CF manifest. It is equivalent to a Namespace in Kubernetes.
	Space string `yaml:"space,omitempty" json:"space,omitempty"`
	// Labels capture the labels as defined in the `annotations` field in the CF application manifest
	Labels map[string]*string `yaml:"labels,omitempty" json:"labels,omitempty"`
	// Annotations capture the annotations as defined in the `labels` field in the CF application manifest
	Annotations map[string]*string `yaml:"annotations,omitempty" json:"annotations,omitempty"`
	// Version captures the version of the manifest containing the resulting CF application manifests list retrieved via REST API.
	Version string `yaml:"version,omitempty" json:"version,omitempty"`
}

type ProcessSpec struct {
	// Type captures the `type` field in the Process specification.
	// Accepted values are `web` or `worker`
	Type ProcessType `yaml:"type" json:"type" validate:"required,oneof=web worker"`

	ProcessSpecTemplate `yaml:",inline" json:",inline" validate:"omitempty"`
}

type ProcessSpecTemplate struct {
	// Command represents the command used to run the process.
	Command string `yaml:"command,omitempty" json:"command,omitempty" validate:"omitempty"`
	// DiskQuota represents the amount of persistent disk requested by the process.
	DiskQuota string `yaml:"disk,omitempty" json:"disk,omitempty" validate:"omitempty"`
	// Memory represents the amount of memory requested by the process.
	Memory string `yaml:"memory,omitempty" json:"memory,omitempty" validate:"omitempty"`
	// HealthCheck captures the health check information
	HealthCheck HealthCheckSpec `yaml:"healthCheck,omitempty" json:"healthCheck,omitempty" validate:"omitempty"`
	// ReadinessCheck captures the readiness check information.
	ReadinessCheck ProbeSpec `yaml:"readinessCheck,omitempty" json:"readinessCheck,omitempty" validate:"omitempty"`
	// Instances represents the number of instances for this process to run.
	Instances int `yaml:"instances,omitempty" json:"instances,omitempty" validate:"omitempty,min=1"`
	// LogRateLimit represents the maximum amount of logs to be captured per second. Defaults to `16K`
	LogRateLimit string `yaml:"logRateLimit,omitempty" json:"logRateLimit,omitempty" validate:"omitempty"`
	// Lifecycle captures the value fo the lifecycle field in the CF application manifest.
	// Valid values are `buildpack`, `cnb`, and `docker`. Defaults to `buildpack`
	Lifecycle LifecycleType `yaml:"lifecycle,omitempty" json:"lifecycle,omitempty" validate:"omitempty,oneof=buildpack cnb docker"`
}

type LifecycleType string

const (
	BuildPackLifecycleType LifecycleType = "buildpack"
	CNBLifecycleType       LifecycleType = "cnb"
	DockerLifecycleType    LifecycleType = "docker"
)

type ProcessType string

const (
	// Web represents a `web` application type
	Web ProcessType = "web"
	// Worker represents a `worker` application type
	Worker ProcessType = "worker"
)

type ProbeSpec struct {
	// Endpoint represents the URL location where to perform the probe check.
	Endpoint string `yaml:"endpoint" json:"endpoint" validate:"omitempty"`
	// InvocationTimeout represents the number of seconds in which the probe check can be considered as timedout.
	// https://docs.cloudfoundry.org/devguide/deploy-apps/manifest-attributes.html#timeout
	InvocationTimeout int `yaml:"invocationTimeout" json:"invocationTimeout" validate:"omitempty,min=0"`
	// Interval represents the number of seconds between probe checks.
	Interval int `yaml:"interval" json:"interval" validate:"omitempty,min=0"`
	// Type specifies the type of health check to perform.
	Type ProbeType `yaml:"type" json:"type" validate:"required,oneof=http process port"`
}

type HealthCheckSpec struct {
	// Inherit the ProbeSpec struct
	ProbeSpec `yaml:",inline" json:",inline" validate:"omitempty"`
	// Timeout specifies the maximum time allowed for an application to
	// respond to readiness or health checks during startup.
	// If the application does not respond within this time, the platform will mark
	// the deployment as failed. The default value is 60 seconds and maximum to 180 seconds, but both values can be changed in the Cloud Foundry Controller.
	// https://github.com/cloudfoundry/docs-dev-guide/blob/96f19d9d67f52ac7418c147d5ddaa79c957eec34/deploy-apps/large-app-deploy.html.md.erb#L35
	// Default is 60 (seconds).
	Timeout int `yaml:"timeout" json:"timeout" validate:"min=0,max=180"`
}

type ProbeType string

const (
	HTTPProbeType    ProbeType = "http"
	ProcessProbeType ProbeType = "process"
	PortProbeType    ProbeType = "port"
)

type Routes []Route

type RouteSpec struct {
	//NoRoute captures the field no-route in the CF Application manifest.
	NoRoute bool `yaml:"noRoute,omitempty" json:"noRoute,omitempty"`
	//RandomRoute captures the field random-route in the CF Application manifest.
	RandomRoute bool `yaml:"randomRoute,omitempty" json:"randomRoute,omitempty"`
	//Routes captures the field routes in the CF Application manifest.
	Routes Routes `yaml:"routes,omitempty" json:"routes,omitempty" validate:"omitempty,dive"`
}

type Route struct {
	// Route captures the domain name, port and path of the route.
	Route string `yaml:"route" json:"route" validate:"required"`
	// Protocol captures the protocol type: http, http2 or tcp. Note that the CF `protocol` field is only available
	// for CF deployments that use HTTP/2 routing.
	Protocol RouteProtocol `yaml:"protocol,omitempty" json:"protocol,omitempty" validate:"omitempty,oneof=http1 http2 tcp"`
	// Options captures the options for the Route. Only load balancing is supported at the moment.
	Options RouteOptions `yaml:"options,omitempty" json:"options,omitempty" validate:"omitempty"`
}

type RouteOptions struct {
	// LoadBalancing captures the settings for load balancing. Only `round-robin` or `least-connection` are supported
	// https://v3-apidocs.cloudfoundry.org/version/3.192.0/index.html#the-route-options-object
	LoadBalancing LoadBalancingType `yaml:"loadBalancing,omitempty" json:"loadBalancing,omitempty" validate:"omitempty,oneof=round-robin least-connection"`
}

type LoadBalancingType string

const (
	RoundRobinLoadBalancingType      LoadBalancingType = "round-robin"
	LeastConnectionLoadBalancingType LoadBalancingType = "least-connection"
)

type RouteProtocol string

const (
	HTTPRouteProtocol  RouteProtocol = "http1"
	HTTP2RouteProtocol RouteProtocol = "http2"
	TCPRouteProtocol   RouteProtocol = "tcp"
)
