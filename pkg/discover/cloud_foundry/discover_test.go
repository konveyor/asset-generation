package cloud_foundry

import (
	"fmt"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var defaultProcessSpecTemplate = ProcessSpecTemplate{
	HealthCheck: ProbeSpec{
		Type:     PortProbeType,
		Endpoint: "/",
		Interval: 30,
		Timeout:  1,
	},
	ReadinessCheck: ProbeSpec{
		Type:     ProcessProbeType,
		Endpoint: "/",
		Interval: 30,
		Timeout:  1,
	},
}
var defaultProcessSpec = ProcessSpec{
	Type:                "",
	ProcessSpecTemplate: defaultProcessSpecTemplate,
}

func overrideWith[T any](defaultVal T, overrides ...func(*T)) *T {
	val := defaultVal
	for _, override := range overrides {
		override(&val)
	}
	return &val
}

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
var _ = Describe("parseProcessSpecs", func() {
	var (
		defaultAppManifest = AppManifest{}
	)

	When("parsing template process", func() {
		It("should return a template with default values if empty", func() {
			template, _, err := parseProcessSpecs(defaultAppManifest)
			Expect(err).NotTo(HaveOccurred())
			Expect(template).ToNot(BeNil())
			Expect(*template).To(Equal(defaultProcessSpecTemplate))
		})

		It("should set LogRateLimit from AppManifest", func() {
			appManifest := AppManifest{
				AppManifestProcess: AppManifestProcess{
					LogRateLimitPerSecond: "32K",
				},
			}
			template, _, err := parseProcessSpecs(appManifest)
			Expect(err).NotTo(HaveOccurred())
			Expect(template.LogRateLimit).To(Equal("32K"))
		})
		It("should set HealtCheck from AppManifest", func() {
			expectedHC := ProbeSpec{
				Type:     HTTPProbeType,
				Endpoint: "/healthcheck",
				Timeout:  42,
				Interval: 10,
			}
			appManifest := AppManifest{
				AppManifestProcess: AppManifestProcess{
					HealthCheckType:              AppHealthCheckType(expectedHC.Type),
					HealthCheckHTTPEndpoint:      expectedHC.Endpoint,
					HealthCheckInterval:          uint(expectedHC.Interval),
					HealthCheckInvocationTimeout: uint(expectedHC.Timeout),
				},
			}
			template, _, err := parseProcessSpecs(appManifest)
			Expect(err).NotTo(HaveOccurred())
			Expect(template.HealthCheck).To(Equal(expectedHC))
		})
		It("should set ReadinessHealthCheck from AppManifest", func() {
			expectedReadinessHC := ProbeSpec{
				Type:     HTTPProbeType,
				Endpoint: "/healthcheck",
				Timeout:  42,
				Interval: 10,
			}
			appManifest := AppManifest{
				AppManifestProcess: AppManifestProcess{
					ReadinessHealthCheckType:         AppHealthCheckType(expectedReadinessHC.Type),
					ReadinessHealthCheckHttpEndpoint: expectedReadinessHC.Endpoint,
					ReadinessHealthCheckInterval:     uint(expectedReadinessHC.Interval),
					ReadinessHealthInvocationTimeout: uint(expectedReadinessHC.Timeout),
				},
			}
			template, _, err := parseProcessSpecs(appManifest)
			Expect(err).NotTo(HaveOccurred())
			Expect(template.ReadinessCheck).To(Equal(expectedReadinessHC))
		})
	})

	When("parsing inline process", func() {
		It("should set Memory to default if not specified", func() {
			appManifest := AppManifest{
				AppManifestProcess: AppManifestProcess{
					Type:                  WebAppProcessType,
					LogRateLimitPerSecond: "32K",
				},
			}
			templateProcess, processSpec, err := parseProcessSpecs(appManifest)

			Expect(err).NotTo(HaveOccurred())
			Expect(templateProcess).To(BeNil())
			Expect(processSpec).ToNot(BeNil())
			Expect(processSpec.Memory).To(Equal("1G")) // Default memory
		})

		It("should set Instances to default if not specified", func() {
			appManifest := AppManifest{
				AppManifestProcess: AppManifestProcess{
					Type: WebAppProcessType,
				},
			}
			templateProcess, processSpec, err := parseProcessSpecs(appManifest)

			Expect(err).NotTo(HaveOccurred())
			Expect(templateProcess).To(BeNil())
			Expect(processSpec).ToNot(BeNil())
			Expect(processSpec.ProcessSpecTemplate.Instances).To(Equal(1)) // Default instances
		})

		It("should set LogRateLimit to default if not specified", func() {
			appManifest := AppManifest{
				AppManifestProcess: AppManifestProcess{
					Type: WebAppProcessType,
				},
			}
			templateProcess, processSpec, err := parseProcessSpecs(appManifest)

			Expect(err).NotTo(HaveOccurred())
			Expect(templateProcess).To(BeNil())
			Expect(processSpec).ToNot(BeNil())
			Expect(processSpec.ProcessSpecTemplate.LogRateLimit).To(Equal("16K"))
		})

		// It("should return an error if marshaling fails", func() {
		// 	// Use a type that cannot be marshaled to JSON
		// 	struct testStruct {
		// 		a string
		// 	}
		// 	appManifest := AppManifest{
		// 		Type: "inline",
		// 		// Add a field that causes marshaling to fail
		// 	}
		// 	_, _, err := parseProcessSpecs(appManifest)
		// 	Expect(err).To(HaveOccurred())
		// })
	})
})

var _ = Describe("Parse Process", func() {

	When("parsing a process", func() {
		DescribeTable("validate the correctness of the parsing logic",
			func(cfApp AppManifest,
				expectedTemplate *ProcessSpecTemplate,
				expectedProcesses *Processes) {
				appManifestProcess, inlineProcess, err := parseProcessSpecs(cfApp)
				Expect(err).ToNot(HaveOccurred())
				if expectedTemplate == nil {
					Expect(*appManifestProcess).To(Equal(defaultProcessSpecTemplate))
				} else {
					Expect(*appManifestProcess).To(Equal(*expectedTemplate))
				}

				if inlineProcess == nil {
					Expect(inlineProcess).To(BeNil())
				} else {
					Expect(*inlineProcess).To(Equal(*expectedProcesses))
				}
			},
			Entry("default values",
				AppManifest{},
				&defaultProcessSpecTemplate,
				nil,
			),
			Entry("with memory only - inline",
				AppManifest{
					AppManifestProcess: AppManifestProcess{
						Memory: "512M",
					},
				},
				overrideWith(defaultProcessSpecTemplate, func(spec *ProcessSpecTemplate) {
					spec.Memory = "512M"
				}),
				nil,
			),
			Entry("with memory only - in processes",
				AppManifest{
					Processes: &AppManifestProcesses{
						AppManifestProcess{
							Type:   WorkerAppProcessType,
							Memory: "512M",
						},
					},
				},
				&defaultProcessSpecTemplate,
				&Processes{
					*overrideWith(defaultProcessSpec, func(spec *ProcessSpec) {
						spec.Memory = "512M"
					}),
				},
			),
			Entry("with instance only - inline",
				AppManifest{
					AppManifestProcess: AppManifestProcess{
						Instances: ptrTo(uint(42)),
					},
				},
				overrideWith(defaultProcessSpecTemplate, func(spec *ProcessSpecTemplate) {
					spec.Instances = 42
				}),
				nil,
			),
			Entry("with instance only - in processes",
				AppManifest{
					Processes: &AppManifestProcesses{
						AppManifestProcess{
							Type:      WorkerAppProcessType,
							Instances: ptrTo(uint(42)),
						},
					},
				},
				nil,
				&Processes{
					*overrideWith(defaultProcessSpec, func(spec *ProcessSpec) {
						spec.Instances = 2
					}),
				},
			),
			Entry("with lograte only - inline",
				AppManifest{
					AppManifestProcess: AppManifestProcess{
						LogRateLimitPerSecond: "42K",
					},
				},
				overrideWith(defaultProcessSpecTemplate, func(spec *ProcessSpecTemplate) {
					spec.LogRateLimit = "42K"
				}),
				nil,
			),
			Entry("with lograte only - in processes",
				AppManifest{
					Processes: &AppManifestProcesses{
						AppManifestProcess{
							Type:                  WorkerAppProcessType,
							LogRateLimitPerSecond: "42K",
						},
					},
				},
				&defaultProcessSpecTemplate,
				&Processes{
					*overrideWith(defaultProcessSpec, func(spec *ProcessSpec) {
						spec.LogRateLimit = "42K"
					}),
				},
			),
		)
	})
})

var _ = Describe("Parse Sidecars", func() {

	When("parsing sidecars", func() {
		DescribeTable("validate the correctness of the parsing logic", func(cfSidecars *AppManifestSideCars, expected Sidecars) {
			result, err := marshalUnmarshal[Sidecars](cfSidecars)
			Expect(err).ToNot(HaveOccurred())
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
						Name: "test-name",
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
						Command: "test-command",
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

var _ = Describe("Parse Services", func() {
	When("parsing the service information", func() {
		DescribeTable("validate the correctness of the parsing logic", func(services AppManifestServices, expected Services) {
			result, err := marshalUnmarshal[Services](services)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(expected))
		},
			Entry("when services are nil", nil, nil),
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

var _ = Describe("Parse metadata", func() {
	When("parsing the metadata information", func() {
		DescribeTable("validate the correctness of the parsing logic", func(metadata AppMetadata, expected Metadata) {
			result, err := Discover(AppManifest{Name: "test-app", Metadata: &metadata})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Metadata).To(Equal(expected))
		},

			Entry("when metadata is nil", nil, Metadata{Name: "test-app"}),
			Entry("when metadata is empty", AppMetadata{}, Metadata{Name: "test-app"}),
			Entry("when labels are provided", AppMetadata{Labels: map[string]*string{"foo": ptrTo("bar")}}, Metadata{Name: "test-app", Labels: map[string]*string{"foo": ptrTo("bar")}}),
			Entry("when annotations are provided", AppMetadata{Annotations: map[string]*string{"bar": ptrTo("foo")}}, Metadata{Name: "test-app", Annotations: map[string]*string{"bar": ptrTo("foo")}}),
			Entry("when all fields are provided",
				AppMetadata{
					Labels:      map[string]*string{"foo": ptrTo("bar")},
					Annotations: map[string]*string{"bar": ptrTo("foo")}},
				Metadata{
					Name:        "test-app",
					Labels:      map[string]*string{"foo": ptrTo("bar")},
					Annotations: map[string]*string{"bar": ptrTo("foo")},
				}),
		)
	})

})

var _ = Describe("Parse Application", func() {
	When("parsing the application information", func() {
		DescribeTable("validate the correctness of the parsing logic", func(app AppManifest, expected Application) {
			result, err := Discover(app)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(expected))
		},
			Entry("when app is empty",
				AppManifest{Name: "test-app"},
				Application{
					Metadata:            Metadata{Name: "test-app"},
					Timeout:             60,
					ProcessSpecTemplate: defaultProcessSpecTemplate,
				},
			),
			Entry("when timeout is set",
				AppManifest{
					Name:               "test-app",
					AppManifestProcess: AppManifestProcess{Timeout: 42},
				},
				Application{
					Metadata: Metadata{Name: "test-app"},
					Routes: RouteSpec{
						NoRoute:     false,
						RandomRoute: false,
						Routes:      nil,
					},
					ProcessSpecTemplate: defaultProcessSpecTemplate,
					Timeout:             42,
				},
			),
			Entry("when instances is set",
				AppManifest{
					Name:               "test-app",
					AppManifestProcess: AppManifestProcess{Instances: ptrTo(uint(2))},
				},
				Application{
					Metadata: Metadata{Name: "test-app"},
					Timeout:  60,
					ProcessSpecTemplate: func() ProcessSpecTemplate {
						p := defaultProcessSpecTemplate
						p.Instances = 2
						return p
					}(),
				},
			),
			Entry("when buildpacks are set",
				AppManifest{
					Name:       "test-app",
					Buildpacks: []string{"foo", "bar"},
				},
				Application{
					Metadata:            Metadata{Name: "test-app"},
					Timeout:             60,
					BuildPacks:          []string{"foo", "bar"},
					ProcessSpecTemplate: defaultProcessSpecTemplate,
				},
			),
			Entry("when environment values are set",
				AppManifest{
					Name: "test-app",
					Env:  map[string]string{"foo": "bar"},
				},
				Application{
					Metadata:            Metadata{Name: "test-app"},
					Timeout:             60,
					Env:                 map[string]string{"foo": "bar"},
					ProcessSpecTemplate: defaultProcessSpecTemplate,
				},
			),
			Entry("when memory is set",
				AppManifest{
					Name:               "test-app",
					AppManifestProcess: AppManifestProcess{Memory: "42G"},
				},
				Application{
					Metadata: Metadata{Name: "test-app"},
					Timeout:  60,
					ProcessSpecTemplate: func() ProcessSpecTemplate {
						p := defaultProcessSpecTemplate // copy the struct value
						p.Memory = "42G"                // override Instances
						return p
					}(),
				},
			),
		)
	})
})

var _ = Describe("Parse docker", func() {
	When("parsing the docker information", func() {
		DescribeTable("validate the correctness of the parsing logic", func(docker AppManifestDocker, expected Docker) {
			result, err := marshalUnmarshal[Docker](docker)
			Expect(err).ToNot(HaveOccurred())
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

var _ = Describe("Validate discover manifest", func() {
	AfterEach(func() {
		os.Remove("manifest.yaml")
	})

	Describe("Application validation", func() {
		Context("when validating buildpacks", func() {

			When("a buildpack list contains an empty entry", func() {
				It("should not return any errors", func() {
					manifestContent := Application{
						Metadata: Metadata{Name: "test-name"},
						ProcessSpecTemplate: ProcessSpecTemplate{
							Instances: 1,
						},
						BuildPacks: []string{"java_buildpack", "", "go_buildpack"},
					}
					validationErrors := validateApplication(manifestContent)
					Expect(validationErrors).To(BeNil(), "Expected no error for valid manifest, but got one")
				})
			})

			When("the buildpacks list is entirely empty", func() {
				It("should not return any errors", func() {
					manifestContent := Application{
						Metadata: Metadata{Name: "test-name"},
						ProcessSpecTemplate: ProcessSpecTemplate{
							Instances: 1,
						},
						BuildPacks: []string{"", ""},
					}
					validationErrors := validateApplication(manifestContent)
					Expect(validationErrors).To(BeNil(), "Expected no error for valid manifest, but got one")
				})
			})

			When("all buildpacks are valid and non-empty", func() {
				It("should not return any errors", func() {
					manifestContent := Application{
						Metadata: Metadata{Name: "test-name"},
						ProcessSpecTemplate: ProcessSpecTemplate{
							Instances: 1,
						},
						BuildPacks: []string{"java_buildpack", "go_buildpack"},
					}
					validationErrors := validateApplication(manifestContent)
					Expect(validationErrors).To(BeNil(), "Expected no error for valid manifest, but got one")
				})
			})
		})

		Context("when validating Docker", func() {

			When("Docker is empty", func() {
				It("should not return any errors", func() {
					manifestContent := Application{
						Metadata: Metadata{Name: "test-name"},
						ProcessSpecTemplate: ProcessSpecTemplate{
							Instances: 1,
						},
						Docker: Docker{},
					}
					validationErrors := validateApplication(manifestContent)
					Expect(validationErrors).To(BeNil(), "Expected no error for valid manifest, but got one")
				})
			})

			When("only the Docker username is provided without an image", func() {
				It("should return a validation error for missing image", func() {
					manifestContent := Application{
						Metadata: Metadata{Name: "test-name"},
						ProcessSpecTemplate: ProcessSpecTemplate{
							Instances: 1,
						},
						Docker: Docker{Username: "foo"},
					}
					expectedErrorMessages := []string{
						generateErrorMessage("Application.Docker.Image", "Image", "required"),
					}

					validationErrors := validateApplication(manifestContent)
					Expect(validationErrors).ToNot(BeNil(), "Expected an error due to missing image, but got none")
					Expect(len(validationErrors)).To(Equal(len(expectedErrorMessages)), "Expected a specific number of validation errors")
					Expect(getValidationErrorMsgs(validationErrors)).To(ConsistOf(expectedErrorMessages),
						"Validation errors did not match expected errors exactly")

				})

				When("both Docker image and username are provided", func() {
					It("should not return any errors", func() {
						manifestContent := Application{
							Metadata: Metadata{Name: "test-name"},
							ProcessSpecTemplate: ProcessSpecTemplate{
								Instances: 1,
							},
							Docker: Docker{Image: "my-app:latest", Username: "foo"},
						}
						validationErrors := validateApplication(manifestContent)
						Expect(validationErrors).To(BeNil(), "Expected no error for valid manifest, but got one")
					})
				})

			})
		})

		Context("when validating env", func() {
			When("when map items are set", func() {
				It("should not return any errors", func() {
					manifestContent := Application{
						Metadata: Metadata{Name: "test-name"},
						ProcessSpecTemplate: ProcessSpecTemplate{
							Instances: 1,
						},
						Env: map[string]string{
							"DATABASE_URL": "postgres://user:pass@localhost:5432/mydb",
							"API_KEY":      "myapikey12345",
						},
					}
					validationErrors := validateApplication(manifestContent)
					Expect(validationErrors).To(BeNil(), "Expected no error for valid manifest, but got one")
				})
			})
		})

		Context("when validating random route", func() {
			When("random route is true", func() {
				It("should not return any errors", func() {
					manifestContent := Application{
						Metadata: Metadata{Name: "test-name"},
						ProcessSpecTemplate: ProcessSpecTemplate{
							Instances: 1,
						},
						Routes: RouteSpec{
							RandomRoute: true,
						},
					}
					validationErrors := validateApplication(manifestContent)
					Expect(validationErrors).To(BeNil(), "Expected no error for valid manifest, but got one")
				})
			})
			Context("when random route is false", func() {
				It("should not return any errors", func() {
					manifestContent := Application{
						Metadata: Metadata{Name: "test-name"},
						ProcessSpecTemplate: ProcessSpecTemplate{
							Instances: 1,
						},
						Routes: RouteSpec{
							RandomRoute: false,
						},
					}
					validationErrors := validateApplication(manifestContent)
					Expect(validationErrors).To(BeNil(), "Expected no error for valid manifest, but got one")
				})
			})
		})
		Context("when validating noroute route", func() {
			Context("when noroute is true", func() {
				It("should not return any errors", func() {
					manifestContent := Application{
						Metadata: Metadata{Name: "test-name"},
						ProcessSpecTemplate: ProcessSpecTemplate{
							Instances: 1,
						},
						Routes: RouteSpec{
							NoRoute: true,
						},
					}
					validationErrors := validateApplication(manifestContent)
					Expect(validationErrors).To(BeNil(), "Expected no error for valid manifest, but got one")
				})
			})

			When("when noroute is false", func() {
				It("should not return any errors", func() {
					manifestContent := Application{
						Metadata: Metadata{Name: "test-name"},
						ProcessSpecTemplate: ProcessSpecTemplate{
							Instances: 1,
						},
						Routes: RouteSpec{
							NoRoute: false,
						},
					}
					validationErrors := validateApplication(manifestContent)
					Expect(validationErrors).To(BeNil(), "Expected no error for valid manifest, but got one")
				})
			})
		})

		Context("when validating routes", func() {

			When("when routes is empty", func() {
				It("should not return any errors", func() {
					manifestContent := Application{
						Metadata: Metadata{Name: "test-name"},
						ProcessSpecTemplate: ProcessSpecTemplate{
							Instances: 1,
						},
						Routes: RouteSpec{
							Routes: Routes{},
						},
					}
					validationErrors := validateApplication(manifestContent)
					Expect(validationErrors).To(BeNil(), "Expected no error for valid manifest, but got one")
				})
			})

			When("routes is nil", func() {
				It("should not return any errors", func() {
					manifestContent := Application{
						Metadata: Metadata{Name: "test-name"},
						ProcessSpecTemplate: ProcessSpecTemplate{
							Instances: 1,
						},
						Routes: RouteSpec{
							Routes: nil,
						},
					}
					validationErrors := validateApplication(manifestContent)
					Expect(validationErrors).To(BeNil(), "Expected no error for valid manifest, but got one")
				})
			})

			When("routes has only options", func() {
				It("should return the correct errors", func() {
					manifestContent := Application{
						Metadata: Metadata{Name: "test-name"},
						ProcessSpecTemplate: ProcessSpecTemplate{
							Instances: 1,
						},
						Routes: RouteSpec{
							Routes: Routes{
								{
									Options: RouteOptions{LoadBalancing: RoundRobinLoadBalancingType},
								},
							},
						},
					}
					expectedErrorMessages := []string{
						generateErrorMessage("Application.Routes.Routes[0].Route", "Route", "required"),
						generateErrorMessage("Application.Routes.Routes[0].Protocol", "Protocol", "required"),
					}

					validationErrors := validateApplication(manifestContent)
					Expect(validationErrors).ToNot(BeNil(), "Expected an error due to invalid manifest content, got none")
					Expect(len(validationErrors)).To(Equal(len(expectedErrorMessages)), "Expected a specific number of validation errors")

					Expect(getValidationErrorMsgs(validationErrors)).To(ConsistOf(expectedErrorMessages),
						"Validation errors did not match expected errors exactly")
				})
			})

			When("routes has only route name", func() {
				It("should return error", func() {
					manifestContent := Application{
						Metadata: Metadata{Name: "test-name"},
						ProcessSpecTemplate: ProcessSpecTemplate{
							Instances: 1,
						},
						Routes: RouteSpec{
							Routes: Routes{
								{
									Route: "http://example.com",
								},
							},
						},
					}
					expectedErrorMessages := []string{
						generateErrorMessage("Application.Routes.Routes[0].Protocol", "Protocol", "required")}

					validationErrors := validateApplication(manifestContent)
					Expect(validationErrors).ToNot(BeNil(), "Expected an error due to invalid manifest content, got none")
					Expect(len(validationErrors)).To(Equal(len(expectedErrorMessages)), "Expected a specific number of validation errors")
					Expect(getValidationErrorMsgs((validationErrors))).To(ConsistOf(expectedErrorMessages),
						fmt.Sprintf("Expected error message '%s' was not found in any validation errors", expectedErrorMessages[0]))
				})
			})

			When("routes has only protocol", func() {
				It("should return error", func() {
					manifestContent := Application{
						Metadata: Metadata{Name: "test-name"},
						ProcessSpecTemplate: ProcessSpecTemplate{
							Instances: 1,
						},
						Routes: RouteSpec{
							Routes: Routes{
								{
									Protocol: HTTPRouteProtocol,
								},
							},
						},
					}
					expectedErrorMessages := []string{
						generateErrorMessage("Application.Routes.Routes[0].Route", "Route", "required")}

					validationErrors := validateApplication(manifestContent)
					Expect(validationErrors).ToNot(BeNil(), "Expected an error due to invalid manifest content, got none")
					Expect(len(validationErrors)).To(Equal(len(expectedErrorMessages)), "Expected a specific number of validation errors")
					Expect(getValidationErrorMsgs(validationErrors)).To(ConsistOf(expectedErrorMessages),
						fmt.Sprintf("Expected error message '%s' was not found in any validation errors", expectedErrorMessages[0]))
				})
			})
			When("routes has name, loadbalancing", func() {
				When("invalid protocol", func() {
					It("should return error", func() {
						manifestContent := Application{
							Metadata: Metadata{Name: "test-name"},
							ProcessSpecTemplate: ProcessSpecTemplate{
								Instances: 1,
							},
							Routes: RouteSpec{
								Routes: Routes{
									{
										Route:    "http://example.com",
										Protocol: "invalid",
										Options:  RouteOptions{LoadBalancing: RoundRobinLoadBalancingType},
									},
								},
							},
						}
						expectedErrorMessages := []string{generateErrorMessage("Application.Routes.Routes[0].Protocol", "Protocol", "oneof")}

						validationErrors := validateApplication(manifestContent)
						Expect(validationErrors).ToNot(BeNil(), "Expected an error due to invalid manifest content, got none")
						Expect(len(validationErrors)).To(Equal(len(expectedErrorMessages)), "Expected a specific number of validation errors")
						Expect(getValidationErrorMsgs(validationErrors)).To(ConsistOf(expectedErrorMessages),
							"Validation errors did not match expected errors exactly")
					})
				})

				When("http1 protocol", func() {
					It("should return the error", func() {
						manifestContent := Application{
							Metadata: Metadata{Name: "test-name"},
							ProcessSpecTemplate: ProcessSpecTemplate{
								Instances: 1,
							},
							Routes: RouteSpec{
								Routes: Routes{
									{
										Route:    "http://example.com",
										Protocol: HTTPRouteProtocol,
										Options:  RouteOptions{LoadBalancing: RoundRobinLoadBalancingType},
									},
								},
							},
						}
						validationErrors := validateApplication(manifestContent)
						Expect(validationErrors).To(BeNil(), "Expected no error for valid manifest, but got one")
					})
				})

				When("http2 protocol", func() {
					It("should not return any errors", func() {
						manifestContent := Application{
							Metadata: Metadata{Name: "test-name"},
							ProcessSpecTemplate: ProcessSpecTemplate{
								Instances: 1,
							},
							Routes: RouteSpec{
								Routes: Routes{
									{
										Route:    "http://example.com",
										Protocol: HTTP2RouteProtocol,
										Options:  RouteOptions{LoadBalancing: RoundRobinLoadBalancingType},
									},
								},
							},
						}
						validationErrors := validateApplication(manifestContent)
						Expect(validationErrors).To(BeNil(), "Expected no error for valid manifest, but got one")
					})
				})

				Context("tcp protocol", func() {
					It("should not return any errors", func() {
						manifestContent := Application{
							Metadata: Metadata{Name: "test-name"},
							ProcessSpecTemplate: ProcessSpecTemplate{
								Instances: 1,
							},
							Routes: RouteSpec{
								Routes: Routes{
									{
										Route:    "http://example.com",
										Protocol: TCPRouteProtocol,
										Options:  RouteOptions{LoadBalancing: RoundRobinLoadBalancingType},
									},
								},
							},
						}
						validationErrors := validateApplication(manifestContent)
						Expect(validationErrors).To(BeNil(), "Expected no error for valid manifest, but got one")
					})
				})
			})
		})
		Context("when validating processes", func() {

			When("when process is empty", func() {
				It("should not return any errors", func() {
					manifestContent := Application{
						Metadata: Metadata{Name: "test-name"},
						ProcessSpecTemplate: ProcessSpecTemplate{
							Instances: 1,
						},
						Processes: Processes{},
					}
					validationErrors := validateApplication(manifestContent)
					Expect(validationErrors).To(BeNil(), "Expected no error for valid manifest, but got one")
				})
			})

			When("when process is nil", func() {
				It("should return errors for missing required fields", func() {
					manifestContent := Application{
						Metadata: Metadata{Name: "test-name"},
						ProcessSpecTemplate: ProcessSpecTemplate{
							Instances: 1,
						},
						Processes: Processes{
							ProcessSpec{},
						},
					}
					expectedErrorMessages := []string{
						generateErrorMessage("Application.Processes[0].Type", "Type", "required"),
					}

					validationErrors := validateApplication(manifestContent)
					Expect(getValidationErrorMsgs(validationErrors)).To(ConsistOf(expectedErrorMessages),
						"Validation errors did not match expected errors exactly")
					Expect(validationErrors).ToNot(BeNil(), "Expected an error due to invalid manifest content, got none")
					Expect(len(validationErrors)).To(Equal(len(expectedErrorMessages)), "Expected a specific number of validation errors")

				})
			})
			When("when process ProbeSpec is empty", func() {
				It("should return errors for missing required fields", func() {
					manifestContent := Application{
						Metadata: Metadata{Name: "test-name"},
						ProcessSpecTemplate: ProcessSpecTemplate{
							Instances: 1,
						},
						Processes: Processes{
							ProcessSpec{
								Type: Web,
								ProcessSpecTemplate: ProcessSpecTemplate{
									Memory:      "50M",
									HealthCheck: ProbeSpec{},
									ReadinessCheck: ProbeSpec{
										Endpoint: "readiness.com",
									},
									Instances:    42,
									LogRateLimit: "10K",
									Lifecycle:    BuildPackLifecycleType,
								},
							},
						},
					}
					expectedErrorMessages := []string{
						generateErrorMessage("Application.Processes[0].ProcessSpecTemplate.ReadinessCheck.Timeout", "Timeout", "required"),
						generateErrorMessage("Application.Processes[0].ProcessSpecTemplate.ReadinessCheck.Interval", "Interval", "required"),
						generateErrorMessage("Application.Processes[0].ProcessSpecTemplate.ReadinessCheck.Type", "Type", "required"),
					}

					validationErrors := validateApplication(manifestContent)
					Expect(getValidationErrorMsgs(validationErrors)).To(ConsistOf(expectedErrorMessages),
						"Validation errors did not match expected errors exactly")
					Expect(validationErrors).ToNot(BeNil(), "Expected an error due to invalid manifest content, got none")
					Expect(len(validationErrors)).To(Equal(len(expectedErrorMessages)), "Expected a specific number of validation errors")

				})
			})
			When("when process ProbeSpec has only endpoint", func() {
				It("should return errors for missing required fields", func() {
					manifestContent := Application{
						Metadata: Metadata{Name: "test-name"},
						ProcessSpecTemplate: ProcessSpecTemplate{
							Instances: 1,
						},
						Processes: Processes{
							ProcessSpec{
								Type: Web,
								ProcessSpecTemplate: ProcessSpecTemplate{
									Memory: "50M",
									HealthCheck: ProbeSpec{
										Endpoint: "healthcheck.com",
									},
									ReadinessCheck: ProbeSpec{
										Endpoint: "readiness.com",
									},
									Instances:    42,
									LogRateLimit: "10K",
									Lifecycle:    BuildPackLifecycleType,
								},
							},
						},
					}
					expectedErrorMessages := []string{
						generateErrorMessage("Application.Processes[0].ProcessSpecTemplate.HealthCheck.Timeout", "Timeout", "required"),
						generateErrorMessage("Application.Processes[0].ProcessSpecTemplate.HealthCheck.Interval", "Interval", "required"),
						generateErrorMessage("Application.Processes[0].ProcessSpecTemplate.HealthCheck.Type", "Type", "required"),
						generateErrorMessage("Application.Processes[0].ProcessSpecTemplate.ReadinessCheck.Timeout", "Timeout", "required"),
						generateErrorMessage("Application.Processes[0].ProcessSpecTemplate.ReadinessCheck.Interval", "Interval", "required"),
						generateErrorMessage("Application.Processes[0].ProcessSpecTemplate.ReadinessCheck.Type", "Type", "required"),
					}

					validationErrors := validateApplication(manifestContent)
					Expect(getValidationErrorMsgs(validationErrors)).To(ConsistOf(expectedErrorMessages),
						"Validation errors did not match expected errors exactly")
					Expect(validationErrors).ToNot(BeNil(), "Expected an error due to invalid manifest content, got none")
					Expect(len(validationErrors)).To(Equal(len(expectedErrorMessages)), "Expected a specific number of validation errors")

				})
			})
		})
	})
})
var _ = Describe("MarshalUnmarshal", func() {
	var (
		inputStruct = struct {
			Name string `json:"name"`
			Age  int    `json:"age"`
		}{
			Name: "John",
			Age:  30,
		}
	)

	When("marshaling and unmarshaling a struct", func() {
		It("should return the same data", func() {
			var output struct {
				Name string `json:"name"`
				Age  int    `json:"age"`
			}
			output, err := marshalUnmarshal[struct {
				Name string `json:"name"`
				Age  int    `json:"age"`
			}](inputStruct)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(Equal(struct {
				Name string `json:"name"`
				Age  int    `json:"age"`
			}{
				Name: "John",
				Age:  30,
			}))
		})
	})
	When("marshaling fails", func() {
		It("should return an error", func() {
			// Use a type that cannot be marshaled to JSON
			_, err := marshalUnmarshal[func()](func() {})
			Expect(err).To(HaveOccurred())
		})
	})
})

// Helper function to create a pointer of a given type
func ptrTo[T comparable](t T) *T {
	return &t
}

func generateErrorMessage(namespace string, field string, tag string) string {
	return fmt.Sprintf("field validation for key '%s' field '%s' failed on the '%s' tag", namespace, field, tag)
}

func getValidationErrorMsgs(validationErrors []error) []string {
	var validationErrorMessages []string
	for _, err := range validationErrors {
		validationErrorMessages = append(validationErrorMessages, err.Error())
	}

	return validationErrorMessages
}
