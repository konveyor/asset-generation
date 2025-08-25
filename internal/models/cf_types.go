package models

// Original source from https://github.com/cloudfoundry/go-cfclient/blob/main/operation/manifest.go

type AppHealthCheckType string

const (
	Http    AppHealthCheckType = "http"
	Port    AppHealthCheckType = "port"
	Process AppHealthCheckType = "process"
)

type AppProcessType string

const (
	WebAppProcessType    AppProcessType = "web"
	WorkerAppProcessType AppProcessType = "worker"
)

type AppRouteProtocol string

const (
	HTTP1 AppRouteProtocol = "http1"
	HTTP2 AppRouteProtocol = "http2"
	TCP   AppRouteProtocol = "tcp"
)

type CloudFoundryManifest struct {
	Version      string         `yaml:"version,omitempty"`
	Space        string         `yaml:"space,omitempty"`
	Applications []*AppManifest `yaml:"applications"`
}

// Metadata allows you to tag API resources with information that does not directly affect its functionality.
type AppMetadata struct {
	Labels      map[string]*string `yaml:"labels,omitempty"`
	Annotations map[string]*string `yaml:"annotations,omitempty"`
}
type AppManifest struct {
	Name               string                `yaml:"name"`
	Buildpacks         []string              `yaml:"buildpacks,omitempty"`
	Docker             *AppManifestDocker    `yaml:"docker,omitempty"`
	Env                map[string]string     `yaml:"env,omitempty"`
	RandomRoute        bool                  `yaml:"random-route,omitempty"`
	NoRoute            bool                  `yaml:"no-route,omitempty"`
	Routes             *AppManifestRoutes    `yaml:"routes,omitempty"`
	Services           *AppManifestServices  `yaml:"services,omitempty"`
	Sidecars           *AppManifestSideCars  `yaml:"sidecars,omitempty"`
	Processes          *AppManifestProcesses `yaml:"processes,omitempty"`
	Stack              string                `yaml:"stack,omitempty"`
	Metadata           *AppMetadata          `yaml:"metadata,omitempty"`
	Path               string                `yaml:"path,omitempty"`
	Features           map[string]bool       `yaml:"features,omitempty"`
	AppManifestProcess `yaml:",inline"`
}

type AppManifestProcesses []AppManifestProcess

type AppManifestProcess struct {
	Type                             AppProcessType     `yaml:"type,omitempty"`
	Command                          string             `yaml:"command,omitempty"`
	DiskQuota                        string             `yaml:"disk_quota,omitempty"`
	HealthCheckType                  AppHealthCheckType `yaml:"health-check-type,omitempty"`
	HealthCheckHTTPEndpoint          string             `yaml:"health-check-http-endpoint,omitempty"`
	HealthCheckInvocationTimeout     uint               `yaml:"health-check-invocation-timeout,omitempty"`
	Instances                        *uint              `yaml:"instances,omitempty"`
	LogRateLimitPerSecond            string             `yaml:"log-rate-limit-per-second,omitempty"`
	Memory                           string             `yaml:"memory,omitempty"`
	Timeout                          int                `yaml:"timeout,omitempty"`
	HealthCheckInterval              uint               `yaml:"health-check-interval,omitempty"`
	ReadinessHealthCheckType         AppHealthCheckType `yaml:"readiness-health-check-type,omitempty"`
	ReadinessHealthCheckHttpEndpoint string             `yaml:"readiness-health-check-http-endpoint,omitempty"`
	ReadinessHealthInvocationTimeout uint               `yaml:"readiness-health-invocation-timeout,omitempty"`
	ReadinessHealthCheckInterval     uint               `yaml:"readiness-health-check-interval,omitempty"`
	Lifecycle                        string             `yaml:"lifecycle,omitempty"`
}

type AppManifestDocker struct {
	Image    string `yaml:"image,omitempty"`
	Username string `yaml:"username,omitempty"`
}

type AppManifestServices []AppManifestService

type AppManifestService struct {
	Name        string                 `yaml:"name"`
	BindingName string                 `yaml:"binding_name,omitempty"`
	Parameters  map[string]interface{} `yaml:"parameters,omitempty"`
}

func (ams *AppManifestService) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var raw interface{}
	if err := unmarshal(&raw); err != nil {
		return err
	}
	switch v := raw.(type) {
	case string:
		ams.Name = v
	case map[interface{}]interface{}:
		for key, value := range v {
			switch key {
			case "name":
				ams.Name = value.(string)
			case "binding_name":
				ams.BindingName = value.(string)
			case "parameters":
				if params, ok := value.(map[interface{}]interface{}); ok {
					ams.Parameters = make(map[string]interface{})
					for k, v := range params {
						if kStr, ok := k.(string); ok {
							ams.Parameters[kStr] = v
						}
					}
				}
			}
		}
	case map[string]interface{}:
		for key, value := range v {
			switch key {
			case "name":
				ams.Name = value.(string)
			case "binding_name":
				ams.BindingName = value.(string)
			case "parameters":
				if params, ok := value.(map[string]interface{}); ok {
					ams.Parameters = params
				}
			}
		}
	}
	return nil
}

type AppManifestRoutes []AppManifestRoute

type AppManifestRoute struct {
	Route    string           `yaml:"route"`
	Protocol AppRouteProtocol `yaml:"protocol,omitempty"`
	Options  *AppRouteOptions `yaml:"options,omitempty"`
}

type AppRouteOptions struct {
	LoadBalancing string `yaml:"loadbalancing"`
}

type AppManifestSideCars []AppManifestSideCar

type AppManifestSideCar struct {
	Name         string           `yaml:"name"`
	ProcessTypes []AppProcessType `yaml:"process_types,omitempty"`
	Command      string           `yaml:"command,omitempty"`
	Memory       string           `yaml:"memory,omitempty"`
}

func NewCloudFoundryManifest(space string, applications ...*AppManifest) *CloudFoundryManifest {
	return &CloudFoundryManifest{
		Version:      "1",
		Space:        space,
		Applications: applications,
	}
}
