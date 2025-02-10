package cloud_foundry

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Health Checks tests", func() {

	When("parsing health check probe", func() {
		defaultProbeSpec := ProbeSpec{
			Type:     PortProbeType,
			Endpoint: "/",
			Timeout:  1,
			Interval: 30,
		}
		overrideDefaultProbeSpec := func(overrides ...func(*ProbeSpec)) ProbeSpec {
			spec := defaultProbeSpec
			for _, override := range overrides {
				override(&spec)
			}
			return spec
		}
		DescribeTable("validate the correctness of the parsing logic", func(app AppManifestProcess, expected ProbeSpec) {
			result := parseHealthCheck(app.HealthCheckType, app.HealthCheckHTTPEndpoint, app.HealthCheckInterval, app.HealthCheckInvocationTimeout)
			// Use Gomega's Expect function for assertions
			Expect(result).To(Equal(expected))
		},
			Entry("with default values",
				AppManifestProcess{},
				defaultProbeSpec),
			Entry("with endpoint only",
				AppManifestProcess{
					HealthCheckHTTPEndpoint: "/example.com",
				},
				overrideDefaultProbeSpec(func(spec *ProbeSpec) {
					spec.Endpoint = "/example.com"
				})),
			Entry("with interval only",
				AppManifestProcess{
					HealthCheckInterval: 42,
				},
				overrideDefaultProbeSpec(func(spec *ProbeSpec) {
					spec.Interval = 42
				})),
			Entry("with timeout only",
				AppManifestProcess{
					HealthCheckInvocationTimeout: 42,
				},
				overrideDefaultProbeSpec(func(spec *ProbeSpec) {
					spec.Timeout = 42
				})),
			Entry("with type only",
				AppManifestProcess{
					HealthCheckType: "http",
				},
				overrideDefaultProbeSpec(func(spec *ProbeSpec) {
					spec.Type = HTTPProbeType
				})),
		)
	})

	When("parsing readiness health check probe", func() {
		defaultProbeSpec := ProbeSpec{
			Type:     ProcessProbeType,
			Endpoint: "/",
			Timeout:  1,
			Interval: 30,
		}
		overrideDefaultProbeSpec := func(overrides ...func(*ProbeSpec)) ProbeSpec {
			spec := defaultProbeSpec
			for _, override := range overrides {
				override(&spec)
			}
			return spec
		}
		DescribeTable("validate the correctness of the parsing logic", func(app AppManifestProcess, expected ProbeSpec) {
			result := parseReadinessHealthCheck(app.ReadinessHealthCheckType, app.ReadinessHealthCheckHttpEndpoint, app.ReadinessHealthCheckInterval, app.ReadinessHealthInvocationTimeout)
			// Use Gomega's Expect function for assertions
			Expect(result).To(Equal(expected))
		},
			Entry("with default values",
				AppManifestProcess{},
				defaultProbeSpec),
			Entry("with type only",
				AppManifestProcess{
					ReadinessHealthCheckType: Http,
				},
				overrideDefaultProbeSpec(func(spec *ProbeSpec) {
					spec.Type = HTTPProbeType
				})),
			Entry("with endpoint only",
				AppManifestProcess{
					ReadinessHealthCheckHttpEndpoint: "/example.com",
				},
				overrideDefaultProbeSpec(func(spec *ProbeSpec) {
					spec.Endpoint = "/example.com"
				})),
			Entry("with interval only",
				AppManifestProcess{
					ReadinessHealthCheckInterval: 42,
				},
				overrideDefaultProbeSpec(func(spec *ProbeSpec) {
					spec.Interval = 42
				})),
			Entry("with timeout only",
				AppManifestProcess{
					ReadinessHealthInvocationTimeout: 42,
				},
				overrideDefaultProbeSpec(func(spec *ProbeSpec) {
					spec.Timeout = 42
				})),
		)
	})
})
var _ = Describe("Parse Process", func() {

	When("parsing a process", func() {
		defaultProcessSpec := ProcessSpec{
			Type:   "",
			Memory: "1G",
			HealthCheck: ProbeSpec{
				Type:     PortProbeType,
				Endpoint: "/",
				Timeout:  1,
				Interval: 30,
			},
			ReadinessCheck: ProbeSpec{
				Type:     ProcessProbeType,
				Endpoint: "/",
				Timeout:  1,
				Interval: 30,
			},
			Instances:    1,
			LogRateLimit: "16K",
		}
		overrideDefaultProcessSpec := func(overrides ...func(*ProcessSpec)) ProcessSpec {
			spec := defaultProcessSpec
			for _, override := range overrides {
				override(&spec)
			}
			return spec
		}

		DescribeTable("validate the correctness of the parsing logic", func(app AppManifestProcess, expected ProcessSpec) {
			result := parseProcess(app)
			Expect(result).To(Equal(expected))
		},
			Entry("default values",
				AppManifestProcess{},
				defaultProcessSpec,
			),
			Entry("with memory only",
				AppManifestProcess{
					Memory: "512M",
				},
				overrideDefaultProcessSpec(func(spec *ProcessSpec) {
					spec.Memory = "512M"
				}),
			),
			Entry("with instance only",
				AppManifestProcess{
					Instances: ptrTo(uint(42)),
				},
				overrideDefaultProcessSpec(func(spec *ProcessSpec) {
					spec.Instances = 42
				}),
			),
			Entry("with only lograte",
				AppManifestProcess{
					LogRateLimitPerSecond: "42K",
				},
				overrideDefaultProcessSpec(func(spec *ProcessSpec) {
					spec.LogRateLimit = "42K"
				}),
			),
		)
	})
	When("parsing a process type", func() {
		DescribeTable("validate the correctness of the parsing logic", func(cfProcessTypes []AppProcessType, expected []ProcessType) {
			result := parseProcessTypes(cfProcessTypes)
			Expect(result).To(Equal(expected))
		},
			Entry("default values with nil input",
				nil,
				[]ProcessType{},
			),
			Entry("default values with empty input",
				[]AppProcessType{},
				[]ProcessType{},
			),
			Entry("with web type",
				[]AppProcessType{WebAppProcessType},
				[]ProcessType{Web},
			),
			Entry("with worker type",
				[]AppProcessType{WorkerAppProcessType},
				[]ProcessType{Worker},
			),
			Entry("with multiple type",
				[]AppProcessType{"web", "worker"},
				[]ProcessType{Web, Worker},
			),
		)
	})
})

var _ = Describe("Parse Sidecars", func() {

	When("parsing sidecars", func() {
		DescribeTable("validate the correctness of the parsing logic", func(cfSidecars *AppManifestSideCars, expected Sidecars) {
			result := parseSidecars(cfSidecars)
			Expect(result).To(Equal(expected))
		},
			Entry("default values with nil input",
				nil,
				nil,
			),
			Entry("default values with empty input",
				&AppManifestSideCars{},
				Sidecars{},
			),
			Entry("one sidecar with only name",
				&AppManifestSideCars{
					AppManifestSideCar{
						Name: "test-name",
					},
				},
				Sidecars{
					{
						Name:         "test-name",
						ProcessTypes: []ProcessType{},
					},
				},
			),
			Entry("one sidecar with only command",
				&AppManifestSideCars{
					AppManifestSideCar{
						Command: "test-command",
					},
				},
				Sidecars{
					{
						Command:      "test-command",
						ProcessTypes: []ProcessType{},
					},
				},
			),
			Entry("one sidecar with only process types",
				&AppManifestSideCars{
					AppManifestSideCar{
						ProcessTypes: []AppProcessType{"web", "worker"},
					},
				},
				Sidecars{
					{
						ProcessTypes: []ProcessType{Web, Worker},
					},
				},
			),
		)
	})
})

var _ = Describe("Parse Routes", func() {

	When("parsing the route information", func() {
		DescribeTable("validate the correctness of the parsing logic for the route specification", func(app AppManifest, expected RouteSpec) {
			result := parseRouteSpec(app.Routes, app.RandomRoute, app.NoRoute)
			Expect(result).To(Equal(expected))
		},
			Entry("when routes are nil, no-route and random-route are false", AppManifest{}, RouteSpec{}),
			Entry("when routes are empty, no-route and random-route are false", AppManifest{Routes: &AppManifestRoutes{}}, RouteSpec{Routes: Routes{}}),
			Entry("when routes are not empty, no-route and random-route are false",
				AppManifest{
					Routes: &AppManifestRoutes{{Route: "foo.bar"}}},
				RouteSpec{
					Routes: Routes{{Route: "foo.bar"}},
				}),
			Entry("when routes are nil, no-route is true and random-route is false",
				AppManifest{
					NoRoute: true,
				},
				RouteSpec{
					NoRoute: true,
				}),
			Entry("when routes have one entry and no-route is true",
				AppManifest{
					NoRoute: true,
					Routes:  &AppManifestRoutes{{Route: "foo.bar"}}},
				RouteSpec{
					NoRoute: true,
				}),
			Entry("when routes are nil, no-route is false and random-route is true",
				AppManifest{
					RandomRoute: true,
				},
				RouteSpec{
					RandomRoute: true,
				}),
			Entry("when routes have two entries, no-route and random-route are false",
				AppManifest{
					Routes: &AppManifestRoutes{{Route: "foo.bar"}, {Route: "bar.foo"}}},
				RouteSpec{
					Routes: Routes{{Route: "foo.bar"}, {Route: "bar.foo"}}},
			),
		)

		DescribeTable("validate the correctness of the parsing logic of the route structure", func(routes AppManifestRoutes, expected Routes) {
			result := parseRoutes(routes)
			Expect(result).To(Equal(expected))
		},
			Entry("when routes are nil", nil, nil),
			Entry("when routes are empty", AppManifestRoutes{}, Routes{}),
			Entry("when routes contain one element with only route field defined", AppManifestRoutes{{Route: "foo.bar"}}, Routes{{Route: "foo.bar"}}),
			Entry("when routes contain one element with only protocol field defined", AppManifestRoutes{{Protocol: HTTP2}}, Routes{{Protocol: HTTP2RouteProtocol}}),
			Entry("when routes contain one element with only options field defined with round-robin load balancing",
				AppManifestRoutes{
					{Options: &AppRouteOptions{LoadBalancing: "round-robin"}}},
				Routes{
					{Options: RouteOptions{LoadBalancing: RoundRobinLoadBalancingType}}}),
			Entry("when routes contain one element with only options field defined with least-connection load balancing",
				AppManifestRoutes{
					{Options: &AppRouteOptions{LoadBalancing: "least-connection"}}},
				Routes{
					{Options: RouteOptions{LoadBalancing: LeastConnectionLoadBalancingType}}}),
			Entry("when routes contain one element with all fields populated",
				AppManifestRoutes{
					{
						Route:    "foo.bar",
						Protocol: TCP,
						Options:  &AppRouteOptions{LoadBalancing: "least-connection"},
					}},
				Routes{
					{
						Route:    "foo.bar",
						Protocol: TCPRouteProtocol,
						Options:  RouteOptions{LoadBalancing: LeastConnectionLoadBalancingType}}}),
			Entry("when routes contain two elements",
				AppManifestRoutes{
					{
						Route:    "foo.bar",
						Protocol: TCP,
						Options:  &AppRouteOptions{LoadBalancing: "round-robin"},
					},
					{
						Route:    "bar.foo",
						Protocol: HTTP1,
					}},
				Routes{
					{
						Route:    "foo.bar",
						Protocol: TCPRouteProtocol,
						Options:  RouteOptions{LoadBalancing: RoundRobinLoadBalancingType}},
					{
						Route:    "bar.foo",
						Protocol: HTTPRouteProtocol,
					}}),
		)
	})

})

var _ = Describe("parse Services", func() {
	When("parsing the service information", func() {
		DescribeTable("validate the correctness of the parsing logic", func(services AppManifestServices, expected Services) {
			result := parseServices(&services)
			Expect(result).To(Equal(expected))
		},
			Entry("when services are nil", nil, Services{}),
			Entry("when services are empty", AppManifestServices{}, Services{}),
			Entry("when one service is provided with only name populated", AppManifestServices{{Name: "foo"}}, Services{{Name: "foo"}}),
			Entry("when one service is provided with parameters provided",
				AppManifestServices{
					{Parameters: map[string]interface{}{"foo": "bar"}},
				},
				Services{
					{Parameters: map[string]interface{}{"foo": "bar"}},
				}),
			Entry("when one service is provided with binding name provided", AppManifestServices{{BindingName: "foo_service"}}, Services{{BindingName: "foo_service"}}),
			Entry("when one service is provided with name, parameters and binding name are provided",
				AppManifestServices{
					{
						Name:        "foo_name",
						Parameters:  map[string]interface{}{"foo": "bar"},
						BindingName: "foo_service",
					},
				},
				Services{
					{
						Name:        "foo_name",
						Parameters:  map[string]interface{}{"foo": "bar"},
						BindingName: "foo_service",
					},
				}),
			Entry("when two services are provided with a unique name populated for each one",
				AppManifestServices{
					{Name: "foo"},
					{Name: "bar"},
				},
				Services{
					{Name: "foo"},
					{Name: "bar"},
				}),
		)
	})
})

var _ = Describe("parse metadata", func() {
	When("parsing the metadata information", func() {
		DescribeTable("validate the correctness of the parsing logic", func(metadata AppMetadata, version, space string, expected Metadata) {
			result, err := Discover(AppManifest{Metadata: &metadata}, version, space)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Metadata).To(Equal(expected))
		},

			Entry("when metadata is nil and version and space are empty", nil, "", "", Metadata{Version: "1"}),
			Entry("when empty metadata, version and space", AppMetadata{}, "", "", Metadata{Version: "1"}),
			Entry("when version is provided", AppMetadata{}, "2", "", Metadata{Version: "2"}),
			Entry("when space is provided", AppMetadata{}, "", "default", Metadata{Version: "1", Space: "default"}),
			Entry("when labels are provided", AppMetadata{Labels: map[string]*string{"foo": ptrTo("bar")}}, "", "", Metadata{Version: "1", Labels: map[string]*string{"foo": ptrTo("bar")}}),
			Entry("when annotations are provided", AppMetadata{Annotations: map[string]*string{"bar": ptrTo("foo")}}, "", "", Metadata{Version: "1", Annotations: map[string]*string{"bar": ptrTo("foo")}}),
			Entry("when all fields are provided",
				AppMetadata{
					Labels:      map[string]*string{"foo": ptrTo("bar")},
					Annotations: map[string]*string{"bar": ptrTo("foo")}},
				"2",
				"default",
				Metadata{
					Labels:      map[string]*string{"foo": ptrTo("bar")},
					Annotations: map[string]*string{"bar": ptrTo("foo")},
					Version:     "2",
					Space:       "default",
				}),
		)
	})

})
var _ = Describe("Parse Application", func() {
	When("parsing the application information", func() {
		DescribeTable("validate the correctness of the parsing logic", func(app AppManifest, version, space string, expected Application) {
			result, err := Discover(app, version, space)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(expected))
		},
			Entry("when app is empty",
				AppManifest{},
				"",
				"",
				Application{
					Metadata:  Metadata{Version: "1"},
					Timeout:   60,
					Instances: 1,
				},
			),
			Entry("when timeout is set",
				AppManifest{
					AppManifestProcess: AppManifestProcess{Timeout: 30},
				},
				"",
				"",
				Application{
					Metadata:  Metadata{Version: "1"},
					Timeout:   30,
					Instances: 1,
				},
			),
			Entry("when instances is set",
				AppManifest{
					AppManifestProcess: AppManifestProcess{Instances: ptrTo(uint(2))},
				},
				"",
				"",
				Application{
					Metadata:  Metadata{Version: "1"},
					Timeout:   60,
					Instances: 2,
				},
			),
			Entry("when buildpacks are set",
				AppManifest{
					Buildpacks: []string{"foo", "bar"},
				},
				"",
				"",
				Application{
					Metadata:   Metadata{Version: "1"},
					Timeout:    60,
					Instances:  1,
					BuildPacks: []string{"foo", "bar"},
				},
			),
			Entry("when environment values are set",
				AppManifest{
					Env: map[string]string{"foo": "bar"},
				},
				"",
				"",
				Application{
					Metadata:  Metadata{Version: "1"},
					Timeout:   60,
					Instances: 1,
					Env:       map[string]string{"foo": "bar"},
				},
			),
			Entry("when all fields are set",
				AppManifest{
					Name:       "foo",
					Buildpacks: []string{"foo", "bar"},
					Docker: &AppManifestDocker{
						Image:    "foo.bar:latest",
						Username: "foo@bar.org",
					},
					RandomRoute: true,
					Routes: &AppManifestRoutes{
						{
							Route:    "foo.bar.org",
							Protocol: HTTP2,
							Options:  &AppRouteOptions{LoadBalancing: "least-connection"},
						},
					},
					Env: map[string]string{"foo": "bar"},
					Services: &AppManifestServices{
						{
							Name:        "foo",
							BindingName: "foo_service",
							Parameters:  map[string]interface{}{"foo": "bar"},
						},
					},
					Sidecars: &AppManifestSideCars{
						{
							Name:         "foo_sidecar",
							ProcessTypes: []AppProcessType{WebAppProcessType, WorkerAppProcessType},
							Command:      "echo hello world",
							Memory:       "2G",
						},
					},
					Stack: "docker",
					Metadata: &AppMetadata{
						Labels:      map[string]*string{"foo": ptrTo("label")},
						Annotations: map[string]*string{"bar": ptrTo("annotation")},
					},
					AppManifestProcess: AppManifestProcess{
						Timeout:   100,
						Instances: ptrTo(uint(5)),
					},
					Processes: &AppManifestProcesses{
						{
							Type:                             WebAppProcessType,
							Command:                          "sleep 100",
							DiskQuota:                        "100M",
							HealthCheckType:                  Http,
							HealthCheckHTTPEndpoint:          "/health",
							HealthCheckInvocationTimeout:     10,
							HealthCheckInterval:              60,
							ReadinessHealthCheckType:         Port,
							ReadinessHealthCheckHttpEndpoint: "localhost:8443",
							ReadinessHealthInvocationTimeout: 99,
							ReadinessHealthCheckInterval:     15,
							Instances:                        ptrTo(uint(2)),
							LogRateLimitPerSecond:            "30k",
							Memory:                           "2G",
							Timeout:                          120,
							Lifecycle:                        "container",
						},
					},
				},
				"2",
				"default",
				Application{
					Metadata: Metadata{
						Version:     "2",
						Name:        "foo",
						Labels:      map[string]*string{"foo": ptrTo("label")},
						Annotations: map[string]*string{"bar": ptrTo("annotation")},
						Space:       "default",
					},
					BuildPacks: []string{"foo", "bar"},
					Stack:      "docker",
					Timeout:    100,
					Instances:  5,
					Env:        map[string]string{"foo": "bar"},
					Routes: RouteSpec{
						RandomRoute: true,
						Routes: Routes{
							{
								Route:    "foo.bar.org",
								Protocol: HTTP2RouteProtocol,
								Options: RouteOptions{
									LoadBalancing: LeastConnectionLoadBalancingType,
								},
							},
						},
					},
					Docker: Docker{
						Image:    "foo.bar:latest",
						Username: "foo@bar.org",
					},
					Services: Services{
						{
							Name:        "foo",
							BindingName: "foo_service",
							Parameters:  map[string]interface{}{"foo": "bar"},
						},
					},
					Sidecars: Sidecars{
						{
							Name:         "foo_sidecar",
							ProcessTypes: []ProcessType{Web, Worker},
							Command:      "echo hello world",
							Memory:       "2G",
						},
					},
					Processes: Processes{
						{
							Type:         Web,
							Command:      "sleep 100",
							DiskQuota:    "100M",
							Instances:    2,
							LogRateLimit: "30k",
							Memory:       "2G",
							Lifecycle:    "container",
							HealthCheck: ProbeSpec{
								Endpoint: "/health",
								Timeout:  10,
								Interval: 60,
								Type:     HTTPProbeType,
							},
							ReadinessCheck: ProbeSpec{
								Endpoint: "localhost:8443",
								Timeout:  99,
								Interval: 15,
								Type:     PortProbeType,
							},
						},
					},
				},
			),
		)
	})
})

var _ = Describe("Parse docker", func() {
	When("parsing the docker information", func() {
		DescribeTable("validate the correctness of the parsing logic", func(docker AppManifestDocker, expected Docker) {
			result := parseDocker(&docker)
			Expect(result).To(Equal(expected))
		},
			Entry("when docker is nil", nil, Docker{}),
			Entry("when docker is empty", AppManifestDocker{}, Docker{}),
			Entry("when docker image is populated", AppManifestDocker{Image: "python3:latest"}, Docker{Image: "python3:latest"}),
			Entry("when docker username is populated", AppManifestDocker{Username: "foo@bar.org"}, Docker{Username: "foo@bar.org"}),
			Entry("when docker image and username are populated",
				AppManifestDocker{
					Image:    "python3:latest",
					Username: "foo@bar.org"},
				Docker{Image: "python3:latest",
					Username: "foo@bar.org"}),
		)
	})
})

// Helper function to create a pointer of a given type
func ptrTo[T comparable](t T) *T {
	return &t
}
