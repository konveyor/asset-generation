package cloud_foundry

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path/filepath"

	"github.com/cloudfoundry/go-cfclient/v3/config"
	"github.com/cloudfoundry/go-cfclient/v3/testutil"
	"github.com/go-logr/logr"
	"github.com/go-logr/stdr"
	cfTypes "github.com/konveyor/asset-generation/internal/models"
	pTypes "github.com/konveyor/asset-generation/pkg/providers/types/provider"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
)

var _ = Describe("CloudFoundry Provider", Ordered, func() {
	AfterAll(func() {
		testutil.Teardown()
	})

	When("performing live connnection", func() {

		Describe("listing apps from Cloud Foundry", func() {
			var (
				g          *testutil.ObjectJSONGenerator
				app1       *testutil.JSONResource
				app2       *testutil.JSONResource
				space      *testutil.JSONResource
				emptySpace *testutil.JSONResource
				serverURL  string
				logger     = logr.New(logr.Discard().GetSink())
			)

			BeforeAll(func() {
				g = testutil.NewObjectJSONGenerator()
				space = g.Space()
				emptySpace = g.Space()
				app1 = g.Application()
				app2 = g.Application()
			})
			Context("when space name doesn't exist", func() {
				BeforeEach(func() {
					serverURL = testutil.SetupMultiple([]testutil.MockRoute{
						{
							Method:      "GET",
							Endpoint:    "/v3/spaces",
							Output:      g.Single(""),
							Status:      http.StatusOK,
							QueryString: "names=" + space.Name + "&" + pagingQueryString,
						},
					}, GlobalT)
				})
				AfterEach(func() {
					testutil.Teardown()
				})
				It("returns	an error", func() {
					cfg, err := config.New(serverURL, config.Token("", "fake-refresh-token"), config.SkipTLSValidation())
					Expect(err).NotTo(HaveOccurred())

					cfConfig := &Config{
						SpaceNames:         []string{space.Name},
						CloudFoundryConfig: cfg,
					}

					p, err := New(cfConfig, &logger, true)
					Expect(err).NotTo(HaveOccurred())
					apps, err := p.ListApps()
					Expect(err).To(HaveOccurred())
					Expect(apps).To(BeNil())

				})
			})
			Context("when apps exist in the space", func() {
				BeforeEach(func() {
					serverURL = testutil.SetupMultiple([]testutil.MockRoute{
						{
							Method:      "GET",
							Endpoint:    "/v3/apps",
							Output:      g.Paged([]string{app1.JSON, app2.JSON}),
							Status:      http.StatusOK,
							QueryString: pagingQueryString + "&space_guids=" + space.GUID,
						},
						{
							Method:      "GET",
							Endpoint:    "/v3/apps",
							Output:      g.Paged([]string{}),
							Status:      http.StatusOK,
							QueryString: pagingQueryString + "&space_guids=" + emptySpace.GUID,
						},
						{
							Method:      "GET",
							Endpoint:    "/v3/spaces",
							Output:      g.Paged([]string{space.JSON}),
							Status:      http.StatusOK,
							QueryString: "names=" + space.Name + "&" + pagingQueryString,
						},
					}, GlobalT)
				})
				AfterEach(func() {
					testutil.Teardown()
				})

				It("returns all the apps in the given space", func() {
					cfg, err := config.New(serverURL, config.Token("", "fake-refresh-token"), config.SkipTLSValidation())
					Expect(err).NotTo(HaveOccurred())

					cfConfig := &Config{
						CloudFoundryConfig: cfg,
						SpaceNames:         []string{space.Name},
					}

					p, err := New(cfConfig, &logger, true)
					Expect(err).NotTo(HaveOccurred())
					apps, err := p.ListApps()
					Expect(err).NotTo(HaveOccurred())
					Expect(apps).To(HaveLen(1))
					Expect(apps).To(HaveKey(space.Name))
					Expect(apps[space.Name]).To(ConsistOf([]AppReference{{SpaceName: space.Name, AppName: app1.Name}, {SpaceName: space.Name, AppName: app2.Name}}))
				})
			})
			Context("when apps don't exist in the space", func() {
				BeforeEach(func() {
					// Create two mock apps in the test server
					serverURL = testutil.SetupMultiple([]testutil.MockRoute{
						{
							Method:      "GET",
							Endpoint:    "/v3/apps",
							Output:      g.Paged([]string{}),
							Status:      http.StatusOK,
							QueryString: pagingQueryString + "&space_guids=" + emptySpace.GUID,
						},
						{
							Method:      "GET",
							Endpoint:    "/v3/spaces",
							Output:      g.Paged([]string{emptySpace.JSON}),
							Status:      http.StatusOK,
							QueryString: "names=" + emptySpace.Name + "&" + pagingQueryString,
						},
					}, GlobalT)
				})
				AfterEach(func() {
					testutil.Teardown()
				})

				It("returns no apps", func() {
					cfg, err := config.New(serverURL, config.Token("", "fake-refresh-token"), config.SkipTLSValidation())
					Expect(err).NotTo(HaveOccurred())

					cfConfig := &Config{
						CloudFoundryConfig: cfg,
						SpaceNames:         []string{emptySpace.Name},
					}

					p, err := New(cfConfig, &logger, true)
					Expect(err).NotTo(HaveOccurred())
					apps, err := p.listAppsFromCloudFoundry()
					Expect(err).NotTo(HaveOccurred())
					Expect(apps).To(HaveLen(1))
					Expect(apps[emptySpace.Name]).To(BeEmpty())
				})
			})
		})

		Context("discovering apps from Cloud Foundry", func() {
			var (
				g          *testutil.ObjectJSONGenerator
				app1       *testutil.JSONResource
				space      *testutil.JSONResource
				emptySpace *testutil.JSONResource
				serverURL  string
				logger     = logr.New(logr.Discard().GetSink())
			)
			BeforeAll(func() {
				g = testutil.NewObjectJSONGenerator()
				space = g.Space()
				emptySpace = g.Space()
				app1 = g.Application()
			})

			When("calling the generateCFManifestFromLiveAPI() function to generate the app manifest from a live connection", func() {
				AfterEach(func() {
					testutil.Teardown()
				})

				DescribeTable("the metadata field", func(metadata cfTypes.AppMetadata) {
					expected := cfTypes.AppManifest{
						Name:     "name",
						Metadata: &metadata,
					}
					m, serverURL := newMockApplication(expected, GlobalT)
					cfg, err := config.New(serverURL, config.Token("", "fake-refresh-token"), config.SkipTLSValidation())
					Expect(err).NotTo(HaveOccurred())
					By("Instantiating a new CF REST API client")
					cfConfig := &Config{
						CloudFoundryConfig: cfg,
						SpaceNames:         []string{m.space().Name},
					}
					p, err := New(cfConfig, &logger, true)
					Expect(err).NotTo(HaveOccurred())
					By("generating the CF manifest from a Live API connection")
					received, err := p.generateCFManifestFromLiveAPI(m.space().Name, m.application().Name)
					Expect(err).NotTo(HaveOccurred())
					By("validating the application discovered contains the expected app data")
					Expect(expected.Name).To(Equal(received.Name))
					Expect(expected.Metadata).To(Equal(received.Metadata))
				},
					Entry("discovers an app with empty values", cfTypes.AppMetadata{}),
					Entry("discovers an app with label values only", cfTypes.AppMetadata{Labels: map[string]*string{"foo": ptrTo("bar")}}),
					Entry("discovers an app with annotation values only", cfTypes.AppMetadata{Annotations: map[string]*string{"foo": ptrTo("bar")}}),
					Entry("discovers an app with label and annotation values",
						cfTypes.AppMetadata{
							Labels:      map[string]*string{"foo": ptrTo("bar"), "lazy": ptrTo("fox")},
							Annotations: map[string]*string{"bar": ptrTo("foo"), "fox": ptrTo("lazy")},
						}),
				)
				DescribeTable("the Env field", func(env map[string]string) {
					expected := cfTypes.AppManifest{
						Name:     "name",
						Metadata: &cfTypes.AppMetadata{},
						Env:      env,
					}
					m, serverURL := newMockApplication(expected, GlobalT)
					cfg, err := config.New(serverURL, config.Token("", "fake-refresh-token"), config.SkipTLSValidation())
					Expect(err).NotTo(HaveOccurred())
					By("Instantiating a new CF REST API client")
					cfConfig := &Config{
						CloudFoundryConfig: cfg,
						SpaceNames:         []string{m.space().Name},
					}
					p, err := New(cfConfig, &logger, true)
					Expect(err).NotTo(HaveOccurred())
					By("generating the CF manifest from a Live API connection")
					received, err := p.generateCFManifestFromLiveAPI(m.space().Name, m.application().Name)
					Expect(err).NotTo(HaveOccurred())
					By("validating the application discovered contains the expected app data")
					Expect(expected.Name).To(Equal(received.Name))
					Expect(expected.Env).To(Equal(received.Env))
				},
					Entry("discovers an app with empty values", map[string]string{}),
					Entry("discovers an app with a few env values",
						map[string]string{
							"foo":              "bar",
							"app_env_settings": `{"foo":"bar"}`,
						}),
				)

				DescribeTable("the Buildpacks field", func(buildpacks []string) {
					expected := cfTypes.AppManifest{
						Name:       "name",
						Metadata:   &cfTypes.AppMetadata{},
						Buildpacks: buildpacks,
					}
					m, serverURL := newMockApplication(expected, GlobalT)
					cfg, err := config.New(serverURL, config.Token("", "fake-refresh-token"), config.SkipTLSValidation())
					Expect(err).NotTo(HaveOccurred())
					By("Instantiating a new CF REST API client")
					cfConfig := &Config{
						CloudFoundryConfig: cfg,
						SpaceNames:         []string{m.space().Name},
					}
					p, err := New(cfConfig, &logger, true)
					Expect(err).NotTo(HaveOccurred())
					By("generating the CF manifest from a Live API connection")
					received, err := p.generateCFManifestFromLiveAPI(m.space().Name, m.application().Name)
					Expect(err).NotTo(HaveOccurred())
					By("validating the application discovered contains the expected app data")
					Expect(expected.Name).To(Equal(received.Name))
					if len(buildpacks) == 0 {
						Expect(received.Buildpacks).To(BeNil())
					} else {
						Expect(received.Buildpacks).To(Equal(expected.Buildpacks))
					}
				},
					Entry("discovers an app with empty values", []string{}),
					Entry("discovers an app with a few values",
						[]string{"java_pack", "ruby_pack"}),
				)

				DescribeTable("the stack field", func(stack string) {
					expected := cfTypes.AppManifest{
						Name:     "name",
						Metadata: &cfTypes.AppMetadata{},
						Stack:    stack,
					}
					m, serverURL := newMockApplication(expected, GlobalT)
					cfg, err := config.New(serverURL, config.Token("", "fake-refresh-token"), config.SkipTLSValidation())
					Expect(err).NotTo(HaveOccurred())
					By("Instantiating a new CF REST API client")
					cfConfig := &Config{
						CloudFoundryConfig: cfg,
						SpaceNames:         []string{m.space().Name},
					}
					p, err := New(cfConfig, &logger, true)
					Expect(err).NotTo(HaveOccurred())
					By("generating the CF manifest from a Live API connection")
					received, err := p.generateCFManifestFromLiveAPI(m.space().Name, m.application().Name)
					Expect(err).NotTo(HaveOccurred())
					By("validating the application discovered contains the expected app data")
					Expect(expected.Name).To(Equal(received.Name))
					if len(stack) == 0 {
						Expect(received.Stack).To(BeEmpty())
					} else {
						Expect(received.Stack).To(Equal(expected.Stack))
					}
				},
					Entry("discovers an app with empty values", ""),
					Entry("discovers an app with a few values", "cflinuxfs4"),
				)

				DescribeTable("the docker field", func(docker *cfTypes.AppManifestDocker) {
					expected := cfTypes.AppManifest{
						Name:     "name",
						Metadata: &cfTypes.AppMetadata{},
						Docker:   docker,
					}
					m, serverURL := newMockApplication(expected, GlobalT)
					cfg, err := config.New(serverURL, config.Token("", "fake-refresh-token"), config.SkipTLSValidation())
					Expect(err).NotTo(HaveOccurred())
					By("Instantiating a new CF REST API client")
					cfConfig := &Config{
						CloudFoundryConfig: cfg,
						SpaceNames:         []string{m.space().Name},
					}
					p, err := New(cfConfig, &logger, true)
					Expect(err).NotTo(HaveOccurred())
					By("generating the CF manifest from a Live API connection")
					received, err := p.generateCFManifestFromLiveAPI(m.space().Name, m.application().Name)
					Expect(err).NotTo(HaveOccurred())
					By("validating the application discovered contains the expected app data")
					Expect(expected.Name).To(Equal(received.Name))
					if docker == nil {
						Expect(received.Docker).To(BeNil())
					} else {
						// Docker's username is not provided for runtime applications
						Expect(received.Docker.Image).To(Equal(expected.Docker.Image))
					}
				},
					Entry("discovers an app with nil value", nil),
					Entry("discovers an app with the image populated", &cfTypes.AppManifestDocker{Image: "python31:latest"}),
					Entry("discovers an app with the image and username populated", &cfTypes.AppManifestDocker{Image: "python31:latest", Username: "anonymous"}),
				)
				DescribeTable("the routes field", func(routes *cfTypes.AppManifestRoutes) {
					expected := cfTypes.AppManifest{
						Name:     "name",
						Metadata: &cfTypes.AppMetadata{},
						Routes:   routes,
						NoRoute:  (routes == nil || (routes != nil && len(*routes) == 0)),
					}
					m, serverURL := newMockApplication(expected, GlobalT)
					cfg, err := config.New(serverURL, config.Token("", "fake-refresh-token"), config.SkipTLSValidation())
					Expect(err).NotTo(HaveOccurred())
					By("Instantiating a new CF REST API client")
					cfConfig := &Config{
						CloudFoundryConfig: cfg,
						SpaceNames:         []string{m.space().Name},
					}
					p, err := New(cfConfig, &logger, true)
					Expect(err).NotTo(HaveOccurred())
					By("generating the CF manifest from a Live API connection")
					received, err := p.generateCFManifestFromLiveAPI(m.space().Name, m.application().Name)
					Expect(err).NotTo(HaveOccurred())
					By("validating the application discovered contains the expected app data")
					Expect(expected.Name).To(Equal(received.Name))
					Expect(received.NoRoute).To(Equal(expected.NoRoute))
					if expected.NoRoute {
						Expect(received.Routes).To(Equal(&cfTypes.AppManifestRoutes{}))
					} else {
						Expect([]cfTypes.AppManifestRoute(*received.Routes)).To(Equal([]cfTypes.AppManifestRoute(*expected.Routes)))
					}
				},
					Entry("discovers an app with no route value and nil routes", nil),
					Entry("discovers an app with no route value and empty routes", &cfTypes.AppManifestRoutes{}),
					Entry("discovers an app with routes defined routes", &cfTypes.AppManifestRoutes{{Route: serverURL, Protocol: cfTypes.HTTP2},
						{Route: "https://foo.bar", Protocol: cfTypes.HTTP2},
					}),
					Entry("discovers an app with routes defined routes and round robin load balancing", &cfTypes.AppManifestRoutes{
						{Route: serverURL, Protocol: cfTypes.HTTP2},
						{Route: "https://foo.bar", Protocol: cfTypes.HTTP2, Options: &cfTypes.AppRouteOptions{LoadBalancing: string(RoundRobinLoadBalancingType)}},
					}),
				)

				DescribeTable("the services field", func(services *cfTypes.AppManifestServices) {
					expected := cfTypes.AppManifest{
						Name:     "name",
						Metadata: &cfTypes.AppMetadata{},
						Services: services,
					}
					m, serverURL := newMockApplication(expected, GlobalT)
					cfg, err := config.New(serverURL, config.Token("", "fake-refresh-token"), config.SkipTLSValidation())
					Expect(err).NotTo(HaveOccurred())
					By("Instantiating a new CF REST API client")
					cfConfig := &Config{
						CloudFoundryConfig: cfg,
						SpaceNames:         []string{m.space().Name},
					}
					p, err := New(cfConfig, &logger, true)
					Expect(err).NotTo(HaveOccurred())
					By("generating the CF manifest from a Live API connection")
					received, err := p.generateCFManifestFromLiveAPI(m.space().Name, m.application().Name)
					Expect(err).NotTo(HaveOccurred())
					By("validating the application discovered contains the expected app data")
					Expect(expected.Name).To(Equal(received.Name))
					if expected.Services == nil {
						Expect(received.Services).To(BeNil())
					} else {
						Expect([]cfTypes.AppManifestService(*received.Services)).To(ContainElements([]cfTypes.AppManifestService(*expected.Services)))
					}
				},
					Entry("discovers an app with no services", nil),
					Entry("discovers an app with no services value and empty value", &cfTypes.AppManifestServices{}),
					Entry("discovers an app with services defined", &cfTypes.AppManifestServices{
						{
							Name:        "service_1",
							BindingName: "binding_service_1",
							Parameters:  map[string]interface{}{"credentials": `{"username":"anonymous","password":"P@ssW0rd"}`, "plan": "xlarge"},
						},
						{
							Name:        "service_2",
							BindingName: "binding_service_2",
						},
					}),
				)

				DescribeTable("the sidecars field", func(sidecars *cfTypes.AppManifestSideCars) {
					expected := cfTypes.AppManifest{
						Name:     "name",
						Metadata: &cfTypes.AppMetadata{},
						Sidecars: sidecars,
					}
					m, serverURL := newMockApplication(expected, GlobalT)
					cfg, err := config.New(serverURL, config.Token("", "fake-refresh-token"), config.SkipTLSValidation())
					Expect(err).NotTo(HaveOccurred())
					By("Instantiating a new CF REST API client")
					cfConfig := &Config{
						CloudFoundryConfig: cfg,
						SpaceNames:         []string{m.space().Name},
					}
					p, err := New(cfConfig, &logger, true)
					Expect(err).NotTo(HaveOccurred())
					By("generating the CF manifest from a Live API connection")
					received, err := p.generateCFManifestFromLiveAPI(m.space().Name, m.application().Name)
					Expect(err).NotTo(HaveOccurred())
					By("validating the application discovered contains the expected app data")
					Expect(expected.Name).To(Equal(received.Name))
					if expected.Sidecars == nil {
						Expect(received.Sidecars).To(BeNil())
					} else {
						Expect([]cfTypes.AppManifestSideCar(*received.Sidecars)).To(BeEquivalentTo([]cfTypes.AppManifestSideCar(*expected.Sidecars)))
					}
				},
					Entry("discovers an app with no sidecars", nil),
					Entry("discovers an app with sidecars defined", &cfTypes.AppManifestSideCars{
						{
							Name:         "sidecar_1",
							ProcessTypes: []cfTypes.AppProcessType{cfTypes.WebAppProcessType, cfTypes.WorkerAppProcessType},
							Command:      "sleep 100",
							Memory:       "100",
						},
						{
							Name:         "sidecar_2",
							ProcessTypes: []cfTypes.AppProcessType{cfTypes.WebAppProcessType},
							Command:      "/bin/sh -c echo 'hello world'",
							Memory:       "1024",
						},
					}),
				)

				DescribeTable("the process field", func(buildpack []string, docker *cfTypes.AppManifestDocker, processes cfTypes.AppManifestProcesses, inline bool) {
					expected := cfTypes.AppManifest{
						Name:       "name",
						Metadata:   &cfTypes.AppMetadata{},
						Buildpacks: buildpack,
						Docker:     docker,
					}
					if len(processes) > 0 {
						if !inline {
							expected.Processes = &processes
						} else {
							p := processes[0]
							b, err := json.Marshal(p)
							Expect(err).NotTo(HaveOccurred())
							Expect(json.Unmarshal(b, &expected)).NotTo(HaveOccurred())
						}
					}
					m, serverURL := newMockApplication(expected, GlobalT)
					cfg, err := config.New(serverURL, config.Token("", "fake-refresh-token"), config.SkipTLSValidation())
					Expect(err).NotTo(HaveOccurred())
					By("Instantiating a new CF REST API client")
					cfConfig := &Config{
						CloudFoundryConfig: cfg,
						SpaceNames:         []string{m.space().Name},
					}
					p, err := New(cfConfig, &logger, true)
					Expect(err).NotTo(HaveOccurred())
					By("generating the CF manifest from a Live API connection")
					received, err := p.generateCFManifestFromLiveAPI(m.space().Name, m.application().Name)
					Expect(err).NotTo(HaveOccurred())
					By("validating the application discovered contains the expected app data")
					Expect(expected.Name).To(Equal(received.Name))
					if inline {
						r := cfTypes.AppManifestProcess{}
						b, err := json.Marshal((*received.Processes)[0])
						Expect(err).NotTo(HaveOccurred())
						Expect(json.Unmarshal(b, &r)).NotTo(HaveOccurred())
						Expect(r).To(Equal(processes[0]))
						// Since we control how many processes we inject here, ensure that the inline process is not
						// duplicated as inline and as a process by accident during the converstion
						Expect(len(*received.Processes)).To(Equal(1))
					} else {
						if expected.Processes == nil {
							Expect(received.Processes).To(BeNil())
						} else if !inline {
							Expect([]cfTypes.AppManifestProcess(*received.Processes)).To(ContainElements([]cfTypes.AppManifestProcess(*expected.Processes)))
						}
					}

				},
					Entry("discovers an app with no processes", nil, nil, nil, false),
					Entry("discovers an app with two proccesses defined for python buildpack lifecycle", []string{"python_buildpack"}, nil, cfTypes.AppManifestProcesses{
						{
							Type:                         cfTypes.WebAppProcessType,
							Command:                      "/bin/echo hello world",
							DiskQuota:                    "1000",
							HealthCheckType:              cfTypes.Http,
							HealthCheckHTTPEndpoint:      "/healthEndpoint",
							HealthCheckInvocationTimeout: 100,
							Instances:                    ptrTo(uint(2)),
							LogRateLimitPerSecond:        "10",
							Memory:                       "1024",
							// Timeout:                          50, Timeout is not available at runtime
							HealthCheckInterval:              120,
							ReadinessHealthCheckType:         cfTypes.Process,
							ReadinessHealthCheckHttpEndpoint: "/readinessCheck",
							ReadinessHealthInvocationTimeout: 150,
							ReadinessHealthCheckInterval:     70,
							Lifecycle:                        "buildpack",
						},
						{
							Type:                         cfTypes.WorkerAppProcessType,
							Command:                      "/bin/echo foo bar",
							HealthCheckType:              cfTypes.Port,
							HealthCheckHTTPEndpoint:      "/healthz",
							HealthCheckInvocationTimeout: 10,
							Instances:                    ptrTo(uint(1)),
							LogRateLimitPerSecond:        "70",
							Memory:                       "2048",
							DiskQuota:                    "200",
							// Timeout:                          500,
							HealthCheckInterval:              20,
							ReadinessHealthCheckType:         cfTypes.Http,
							ReadinessHealthCheckHttpEndpoint: "/readinez",
							ReadinessHealthInvocationTimeout: 10,
							ReadinessHealthCheckInterval:     730,
							Lifecycle:                        "buildpack",
						},
					}, false),
					Entry("discovers an app with two proccesses defined for docker lifecycle", nil, &cfTypes.AppManifestDocker{Image: "pyton31:latest"}, cfTypes.AppManifestProcesses{
						{
							Type:                         cfTypes.WebAppProcessType,
							Command:                      "/bin/echo hello world",
							DiskQuota:                    "1000",
							HealthCheckType:              cfTypes.Http,
							HealthCheckHTTPEndpoint:      "/healthEndpoint",
							HealthCheckInvocationTimeout: 100,
							Instances:                    ptrTo(uint(2)),
							LogRateLimitPerSecond:        "10",
							Memory:                       "1024",
							// Timeout:                          50, Timeout is not available at runtime
							HealthCheckInterval:              120,
							ReadinessHealthCheckType:         cfTypes.Process,
							ReadinessHealthCheckHttpEndpoint: "/readinessCheck",
							ReadinessHealthInvocationTimeout: 150,
							ReadinessHealthCheckInterval:     70,
							Lifecycle:                        "docker",
						},
						{
							Type:                         cfTypes.WorkerAppProcessType,
							Command:                      "/bin/echo foo bar",
							HealthCheckType:              cfTypes.Port,
							HealthCheckHTTPEndpoint:      "/healthz",
							HealthCheckInvocationTimeout: 10,
							Instances:                    ptrTo(uint(1)),
							LogRateLimitPerSecond:        "70",
							Memory:                       "2048",
							DiskQuota:                    "200",
							// Timeout:                          500,
							HealthCheckInterval:              20,
							ReadinessHealthCheckType:         cfTypes.Http,
							ReadinessHealthCheckHttpEndpoint: "/readinez",
							ReadinessHealthInvocationTimeout: 10,
							ReadinessHealthCheckInterval:     730,
							Lifecycle:                        "docker",
						},
					}, false),
					Entry("discovers an app with one process inline", []string{"python_buildpack"}, nil, cfTypes.AppManifestProcesses{
						{
							Type:                         cfTypes.WebAppProcessType,
							Command:                      "/bin/echo hello world",
							DiskQuota:                    "1000",
							HealthCheckType:              cfTypes.Http,
							HealthCheckHTTPEndpoint:      "/healthEndpoint",
							HealthCheckInvocationTimeout: 100,
							Instances:                    ptrTo(uint(2)),
							LogRateLimitPerSecond:        "10",
							Memory:                       "1024",
							// Timeout:                          50,
							HealthCheckInterval:              120,
							ReadinessHealthCheckType:         cfTypes.Process,
							ReadinessHealthCheckHttpEndpoint: "/readinessCheck",
							ReadinessHealthInvocationTimeout: 150,
							ReadinessHealthCheckInterval:     70,
							Lifecycle:                        "buildpack",
						},
					}, true),
				)

				It("discover an app fully defined app", func() {
					expected := cfTypes.AppManifest{
						Name: "name",
						Env:  map[string]string{"fox": "lazy"},
						Metadata: &cfTypes.AppMetadata{
							Labels: map[string]*string{"foo": ptrTo("bar")},
						},
						Docker: &cfTypes.AppManifestDocker{
							Image: "docker_image",
						},
						Routes: &cfTypes.AppManifestRoutes{
							{
								Route:    "https://route1",
								Protocol: cfTypes.HTTP2,
							},
						},
						Services: &cfTypes.AppManifestServices{
							{
								Name:        "service_A",
								BindingName: "binding_service_A",
								Parameters: map[string]interface{}{
									"credentials": `{"username":"anonymous","password":"P@ssW0rd"}`,
								},
							},
						},
						Sidecars: &cfTypes.AppManifestSideCars{
							{
								Name:         "sidecar_A",
								ProcessTypes: []cfTypes.AppProcessType{cfTypes.WebAppProcessType, cfTypes.WorkerAppProcessType},
								Command:      "/bin/sleep 1000",
								Memory:       "100",
							},
						},
						Processes: &cfTypes.AppManifestProcesses{
							{
								Type:                             cfTypes.WebAppProcessType,
								Command:                          "/bin/echo hello world",
								DiskQuota:                        "1000",
								HealthCheckType:                  cfTypes.Http,
								HealthCheckHTTPEndpoint:          "/healthEndpoint",
								HealthCheckInvocationTimeout:     100,
								Instances:                        ptrTo(uint(2)),
								LogRateLimitPerSecond:            "10",
								Memory:                           "1024",
								HealthCheckInterval:              120,
								ReadinessHealthCheckType:         cfTypes.Process,
								ReadinessHealthCheckHttpEndpoint: "/readinessCheck",
								ReadinessHealthInvocationTimeout: 150,
								ReadinessHealthCheckInterval:     70,
								Lifecycle:                        "docker",
							},
						},
						Stack: "cfLinux65",
					}
					m, serverURL := newMockApplication(expected, GlobalT)
					cfg, err := config.New(serverURL, config.Token("", "fake-refresh-token"), config.SkipTLSValidation())
					Expect(err).NotTo(HaveOccurred())
					By("Instantiating a new CF REST API client")
					cfConfig := &Config{
						CloudFoundryConfig: cfg,
						SpaceNames:         []string{m.space().Name},
					}
					p, err := New(cfConfig, &logger, true)
					Expect(err).NotTo(HaveOccurred())
					By("discovering the application")
					received, err := p.generateCFManifestFromLiveAPI(m.space().Name, m.application().Name)
					Expect(err).NotTo(HaveOccurred())
					By("validating the application discovered contains the expected app data")
					Expect(*received).To(Equal(expected))
				})

			})
			Context("when apps exist in the space", func() {
				BeforeEach(func() {
					serverURL = testutil.SetupMultiple([]testutil.MockRoute{
						{
							Method:      "GET",
							Endpoint:    "/v3/apps",
							Output:      g.Paged([]string{app1.JSON}),
							Status:      http.StatusOK,
							QueryString: "names=" + app1.Name + "&" + pagingQueryString + "&space_guids=" + space.GUID,
						},
						{
							Method:      "GET",
							Endpoint:    "/v3/apps/" + app1.GUID + "/env",
							Output:      g.Paged([]string{}),
							Status:      http.StatusOK,
							QueryString: "",
						},
						{
							Method:      "GET",
							Endpoint:    "/v3/apps/" + app1.GUID + "/processes",
							Output:      g.Paged([]string{}),
							Status:      http.StatusOK,
							QueryString: pagingQueryString,
						},
						{
							Method:      "GET",
							Endpoint:    "/v3/apps/" + app1.GUID + "/routes",
							Output:      g.Paged([]string{}),
							Status:      http.StatusOK,
							QueryString: pagingQueryString,
						},
						{
							Method:      "GET",
							Endpoint:    "/v3/apps/" + app1.GUID + "/sidecars",
							Output:      g.Paged([]string{}),
							Status:      http.StatusOK,
							QueryString: pagingQueryString,
						},
						{
							Method:      "GET",
							Endpoint:    "/v3/spaces",
							Output:      g.Paged([]string{space.JSON}),
							Status:      http.StatusOK,
							QueryString: "names=" + space.Name + "&" + pagingQueryString,
						},
					}, GlobalT)
				})
				AfterEach(func() {
					testutil.Teardown()
				})

				It("discovers an app with empty spec and only its name and GUID defined", func() {
					cfg, err := config.New(serverURL, config.Token("", "fake-refresh-token"), config.SkipTLSValidation())
					Expect(err).NotTo(HaveOccurred())

					cfConfig := &Config{
						CloudFoundryConfig: cfg,
					}

					p, err := New(cfConfig, &logger, true)
					Expect(err).NotTo(HaveOccurred())
					apps, err := p.Discover(AppReference{SpaceName: space.Name, AppName: app1.Name})
					Expect(err).NotTo(HaveOccurred())
					Expect(apps).NotTo(Equal(&pTypes.DiscoverResult{}))
				})
			})

			Context("when apps don't exist in the space", func() {
				BeforeEach(func() {
					serverURL = testutil.SetupMultiple([]testutil.MockRoute{
						{
							Method:      "GET",
							Endpoint:    "/v3/apps",
							Output:      g.Paged([]string{}),
							Status:      http.StatusOK,
							QueryString: pagingQueryString + "&space_guids=" + emptySpace.GUID,
						},
						{
							Method:      "GET",
							Endpoint:    "/v3/spaces",
							Output:      g.Paged([]string{emptySpace.JSON}),
							Status:      http.StatusOK,
							QueryString: "names=" + emptySpace.Name + "&" + pagingQueryString,
						},
					}, GlobalT)
				})
				AfterEach(func() {
					testutil.Teardown()
				})

				It("returns no apps", func() {
					cfg, err := config.New(serverURL, config.Token("", "fake-refresh-token"), config.SkipTLSValidation())
					Expect(err).NotTo(HaveOccurred())

					cfConfig := &Config{
						CloudFoundryConfig: cfg,
						SpaceNames:         []string{emptySpace.Name},
					}

					p, err := New(cfConfig, &logger, true)
					Expect(err).NotTo(HaveOccurred())
					apps, err := p.listAppsFromCloudFoundry()
					Expect(err).NotTo(HaveOccurred())
					Expect(apps).To(HaveLen(1))
					Expect(apps[emptySpace.Name]).To(BeEmpty())
				})
			})
		})

	})

	When("performing local discovery", func() {

		Describe("listAppsFromLocalManifests", func() {
			var (
				provider  *CloudFoundryProvider
				nopLogger = logr.New(logr.Discard().GetSink())
			)

			BeforeEach(func() {
			})

			Context("when manifest path is a directory with multiple manifests", func() {
				BeforeEach(func() {
					provider = &CloudFoundryProvider{
						cfg: &Config{
							ManifestPath: filepath.Join("./test_data", "multiple_manifests"),
						},
						logger: &nopLogger,
					}
				})

				It("returns app names from manifests in the directory (ignoring subfolders and non-yaml files)", func() {
					apps, err := provider.listAppsFromLocalManifests()
					Expect(err).NotTo(HaveOccurred())
					Expect(apps).To(HaveKey("local"))
					localApps, ok := apps["local"]
					Expect(ok).To(BeTrue())

					appSlice := make([]AppReference, 0)
					for _, app := range localApps {
						appRef, ok := app.(AppReference)
						Expect(ok).To(BeTrue())
						Expect(appRef).ToNot(Equal(AppReference{}))
						appSlice = append(appSlice, appRef)
					}

					Expect(appSlice).To(ContainElements(AppReference{AppName: "app1"}, AppReference{AppName: "app2"}, AppReference{AppName: "app3"}))
					Expect(appSlice).NotTo(ContainElement("app-in-subfolder"))
					Expect(appSlice).NotTo(ContainElement("text-file"))
				})

				It("logs an error and continues when manifest files contain invalid YAML", func() {
					logBuf := new(bytes.Buffer)
					stdLogger := log.New(logBuf, "", 0)
					logger := stdr.New(stdLogger)

					provider = &CloudFoundryProvider{
						cfg: &Config{
							ManifestPath: filepath.Join("./test_data", "invalid_manifest"),
						},
						logger: &logger,
					}
					apps, err := provider.listAppsFromLocalManifests()
					Expect(err).ToNot(HaveOccurred())
					Expect(apps).ToNot(BeNil())
					Expect(apps).To(HaveLen(1))
					Expect(apps).To(HaveKey("local"))
					Expect(apps["local"]).To(HaveLen(0))
					logOutput := logBuf.String()
					Expect(logOutput).To(ContainSubstring("error processing manifest file"))
				})
				It("logs a warning and skips manifests missing app name", func() {
					logBuf := new(bytes.Buffer)
					stdLogger := log.New(logBuf, "", 0)
					logger := stdr.New(stdLogger)

					provider = &CloudFoundryProvider{
						cfg: &Config{
							ManifestPath: filepath.Join("./test_data", "no_app_name_manifest"),
						},
						logger: &logger,
					}
					apps, err := provider.listAppsFromLocalManifests()
					Expect(err).ToNot(HaveOccurred())
					Expect(apps).ToNot(BeNil())
					Expect(apps).To(HaveLen(1))
					Expect(apps).To(HaveKey("local"))
					Expect(apps["local"]).To(HaveLen(0))
					logOutput := logBuf.String()
					Expect(logOutput).To(ContainSubstring(" does not contain an app name"))
				})
			})

			Context("when manifest path is a single manifest file", func() {
				BeforeEach(func() {
					provider = &CloudFoundryProvider{
						cfg: &Config{
							ManifestPath: filepath.Join("./test_data", "test-app", "manifest.yml"),
						},
						logger: &nopLogger,
					}
				})

				It("returns the app name from the single manifest file", func() {
					apps, err := provider.listAppsFromLocalManifests()
					Expect(err).NotTo(HaveOccurred())

					localApp, ok := apps["local"]
					Expect(ok).To(BeTrue())
					Expect(localApp).To(HaveLen(1))

					var appRef AppReference
					for _, app := range localApp {
						appRef, ok = app.(AppReference)
						Expect(ok).To(BeTrue())
						Expect(appRef).ToNot(Equal(AppReference{}))
					}
					Expect(appRef.AppName).To(Equal("my-app"))
				})
			})
		})

		Describe("discoverFromManifestFile", func() {
			var (
				provider     *CloudFoundryProvider
				manifestPath string
				nopLogger    = logr.New(logr.Discard().GetSink())
				err          error
			)
			Context("when it's a single file", func() {
				BeforeEach(func() {
					manifestPath = filepath.Join("test_data", "test-app", "manifest.yml")
					provider, err = New(&Config{ManifestPath: manifestPath}, &nopLogger, true)
					Expect(err).NotTo(HaveOccurred())
				})

				It("successfully parses a valid manifest and returns an Application", func() {
					app, err := provider.discoverFromManifestFile(manifestPath)
					Expect(err).ToNot(HaveOccurred())
					Expect(app).ToNot(BeNil())
					Expect(app.Metadata).ToNot(BeNil())
					Expect(app.Metadata.Name).To(Equal("my-app"))
				})

				It("returns an error if the manifest file does not exist", func() {
					app, err := provider.discoverFromManifestFile("/not/exist/manifest")
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("failed to read manifest file"))
					Expect(app).To(BeNil())
				})

				It("returns an error if the manifest YAML is invalid", func() {
					invalidManifestPath := filepath.Join("test_data", "invalid_manifest", "manifest.yml")
					app, err := provider.discoverFromManifestFile(invalidManifestPath)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("failed to unmarshal YAML"))
					Expect(app).To(BeNil())
				})

				It("returns an error if parseCFApp fails", func() {
					originalParseCFApp := parseCFApp
					DeferCleanup(func() {
						parseCFApp = originalParseCFApp
					})
					mockParseCF := func(spaceName string, manifest cfTypes.AppManifest) (Application, error) {
						return Application{}, fmt.Errorf("mock parse error")
					}
					parseCFApp = mockParseCF

					app, err := provider.discoverFromManifestFile(manifestPath)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("failed to create application"))
					Expect(app).To(BeNil())
				})
				It("parses correctly the probes from an inlined process spec", func() {
					expected := Application{
						Metadata: Metadata{Name: "app-with-inline-process"},
						Docker:   Docker{Image: "myregistry/myapp:latest"},
						Processes: Processes{
							{
								Type: Web,
								ProcessSpecTemplate: ProcessSpecTemplate{
									LogRateLimit: "16K",
									Instances:    2,
									Memory:       "500M",
									ReadinessCheck: ProbeSpec{
										Endpoint: "/readiness",
										Interval: 60,
										Timeout:  10,
										Type:     ProcessProbeType,
									},
									HealthCheck: ProbeSpec{
										Endpoint: "/health",
										Interval: 90,
										Timeout:  3,
										Type:     PortProbeType,
									},
								},
							},
						},
						Timeout:             60,
						ProcessSpecTemplate: &ProcessSpecTemplate{Instances: 1},
					}
					processManifestPath := filepath.Join("test_data", "process_manifest", "manifest.yml")
					app, err := provider.discoverFromManifestFile(processManifestPath)
					Expect(err).NotTo(HaveOccurred())
					Expect(app).To(BeEquivalentTo(&expected))
				})

				It("parses correctly the probes when only type is defined for an inline process", func() {
					expected := Application{
						Metadata: Metadata{Name: "app-with-inline-process-only-type"},
						Docker: Docker{
							Image:    "myregistry/myapp:latest",
							Username: "docker-registry-user"},
						Processes: Processes{
							{
								Type:    Web,
								Timeout: 10,
								ProcessSpecTemplate: ProcessSpecTemplate{
									LogRateLimit: "16K",
									Instances:    1,
									Memory:       "500M",
									DiskQuota:    "512M",
									ReadinessCheck: ProbeSpec{
										Endpoint: "/",
										Interval: 30,
										Timeout:  1,
										Type:     ProcessProbeType,
									},
									HealthCheck: ProbeSpec{
										Endpoint: "/",
										Interval: 30,
										Timeout:  1,
										Type:     PortProbeType,
									},
								},
							},
						},
						Timeout:             60,
						ProcessSpecTemplate: &ProcessSpecTemplate{Instances: 1},
					}
					processManifestPath := filepath.Join("test_data", "inline-process-with-type-only-manifest", "manifest.yml")
					app, err := provider.discoverFromManifestFile(processManifestPath)
					Expect(err).NotTo(HaveOccurred())
					Expect(app).To(BeEquivalentTo(&expected))
				})
				It("validates the discovery data of an app with random route and path", func() {
					expected := Application{
						Metadata:            Metadata{Name: "hello-spring-cloud"},
						ProcessSpecTemplate: &ProcessSpecTemplate{Instances: 1},
						Path:                "target/hello-spring-cloud-0.0.1.BUILD-SNAPSHOT.jar",
						Timeout:             60,
						Routes: RouteSpec{
							RandomRoute: true,
						},
					}
					processManifestPath := filepath.Join("test_data", "hello-spring-cloud", "manifest.yml")
					app, err := provider.discoverFromManifestFile(processManifestPath)
					Expect(err).NotTo(HaveOccurred())
					Expect(app).To(BeEquivalentTo(&expected))
				})
				It("validates the discovery data of an app with only a service", func() {
					expected := Application{
						Metadata:            Metadata{Name: "sailspong"},
						ProcessSpecTemplate: &ProcessSpecTemplate{Instances: 1},
						Timeout:             60,
						Services: Services{
							{Name: "mysql"},
						},
					}
					processManifestPath := filepath.Join("test_data", "pong-matcher-sails", "manifest.yml")
					app, err := provider.discoverFromManifestFile(processManifestPath)
					Expect(err).NotTo(HaveOccurred())
					Expect(app).To(BeEquivalentTo(&expected))
				})
				It("validates the discovery data of an app with a service, command and path", func() {
					expected := Application{
						Metadata: Metadata{Name: "rails-sample"},
						ProcessSpecTemplate: &ProcessSpecTemplate{
							Instances: 1,
							Command:   "bundle exec rake db:migrate && bundle exec rails s -p $PORT",
							Memory:    "256M",
						},
						Timeout: 60,
						Routes: RouteSpec{
							RandomRoute: true,
						},
						Services: Services{
							{Name: "rails-postgres"},
						},
						Path: ".",
					}
					processManifestPath := filepath.Join("test_data", "rails-sample-app", "manifest.yml")
					app, err := provider.discoverFromManifestFile(processManifestPath)
					Expect(err).NotTo(HaveOccurred())
					Expect(app).To(BeEquivalentTo(&expected))
				})
				It("validates the discovery data of an app with features", func() {
					expected := Application{
						Metadata: Metadata{Name: "app-features"},
						ProcessSpecTemplate: &ProcessSpecTemplate{
							Instances: 1,
						},
						Routes: RouteSpec{
							NoRoute: true,
						},
						Timeout: 60,
						Features: map[string]bool{
							"ssh":                      true,
							"revisions":                true,
							"service-binding-k8s":      false,
							"file-based-vcap-services": false,
						},
					}
					processManifestPath := filepath.Join("test_data", "app-features", "manifest.yml")
					app, err := provider.discoverFromManifestFile(processManifestPath)
					Expect(err).NotTo(HaveOccurred())
					Expect(app).To(BeEquivalentTo(&expected))
				})
				It("validates the discovery data of an app with a sidecar", func() {
					expected := Application{
						Metadata: Metadata{Name: "sidecar-dependent-app"},
						ProcessSpecTemplate: &ProcessSpecTemplate{
							Instances: 1,
							Memory:    "256M",
							DiskQuota: "1G",
						},
						Env: map[string]string{
							"CONFIG_SERVER_PORT": "8082",
						},
						Stack:   "cflinuxfs3",
						Timeout: 60,
						Sidecars: Sidecars{
							{
								Name:         "config-server",
								ProcessTypes: []ProcessType{"web"},
								Command:      "./config-server",
							},
						},
					}
					processManifestPath := filepath.Join("test_data", "sidecar-dependant-app", "manifest.yml")
					app, err := provider.discoverFromManifestFile(processManifestPath)
					Expect(err).NotTo(HaveOccurred())
					Expect(app).To(BeEquivalentTo(&expected))
				})
				It("validates the discovery data of an app with service, route and protocol in route", func() {
					expected := Application{
						Metadata: Metadata{Name: "spring-music"},
						ProcessSpecTemplate: &ProcessSpecTemplate{
							Instances: 1,
							Memory:    "1G",
							DiskQuota: "1G",
						},
						Env: map[string]string{
							"JBP_CONFIG_SPRING_AUTO_RECONFIGURATION": "{enabled: false}",
							"SPRING_PROFILES_ACTIVE":                 "http2",
							"JBP_CONFIG_OPEN_JDK_JRE":                "{ jre: { version: 17.+ } }",
						},
						BuildPacks: []string{"java_buildpack"},
						Path:       "build/libs/spring-music-1.0.jar",
						Timeout:    60,
						Routes: RouteSpec{
							Routes: Routes{
								{
									Route:    "rammstein.music",
									Protocol: HTTP2RouteProtocol,
								},
							},
						},
						Services: Services{
							{
								Name: "mysql",
							},
							{
								Name:       "gateway",
								Parameters: map[string]any{"routes": map[string]any{"path": "/music/**"}},
							},
							{
								Name:        "lb",
								BindingName: "load_balancer",
							},
						},
					}
					processManifestPath := filepath.Join("test_data", "spring-music", "manifest.yml")
					app, err := provider.discoverFromManifestFile(processManifestPath)
					Expect(err).NotTo(HaveOccurred())
					Expect(app).To(BeEquivalentTo(&expected))
				})
				It("validates the discovery data of an app with multiple processes", func() {
					expected := Application{
						Metadata: Metadata{Name: "multiple-processes"},
						ProcessSpecTemplate: &ProcessSpecTemplate{
							Instances: 1,
						},
						Timeout: 60,
						Routes: RouteSpec{
							RandomRoute: true,
						},
						Processes: Processes{
							{
								Type:    Web,
								Timeout: 10,
								ProcessSpecTemplate: ProcessSpecTemplate{
									Command:   "start-web.sh",
									DiskQuota: "512M",
									HealthCheck: ProbeSpec{
										Endpoint: "/healthcheck",
										Timeout:  10,
										Type:     HTTPProbeType,
										Interval: 30,
									},
									ReadinessCheck: ProbeSpec{
										Endpoint: "/",
										Interval: 30,
										Timeout:  1,
										Type:     ProcessProbeType,
									},
									Instances:    3,
									Memory:       "500M",
									LogRateLimit: "16K",
								},
							},
							{
								Type:    Worker,
								Timeout: 15,
								ProcessSpecTemplate: ProcessSpecTemplate{
									Command:   "start-worker.sh",
									DiskQuota: "1G",
									Instances: 2,
									Memory:    "256M",
									HealthCheck: ProbeSpec{
										Type:     ProcessProbeType,
										Endpoint: "/",
										Interval: 30,
										Timeout:  1,
									},
									ReadinessCheck: ProbeSpec{
										Endpoint: "/",
										Timeout:  1,
										Interval: 30,
										Type:     ProcessProbeType,
									},
									LogRateLimit: "16K",
								},
							},
						},
					}
					processManifestPath := filepath.Join("test_data", "multiple-processes", "manifest.yml")
					app, err := provider.discoverFromManifestFile(processManifestPath)
					Expect(err).NotTo(HaveOccurred())
					Expect(app).To(BeEquivalentTo(&expected))
				})
				It("validates the discovery data of an app with env, services, processes, routes and sidecars ", func() {
					expected := Application{
						Metadata: Metadata{
							Name: "complete",
							Annotations: map[string]*string{
								"contact": ptrTo("bob@example.com jane@example.com"),
							},
							Labels: map[string]*string{
								"sensitive": ptrTo("true"),
							},
						},
						BuildPacks: []string{
							"ruby_buildpack",
							"java_buildpack",
						},
						Env: map[string]string{
							"VAR1": "value1",
							"VAR2": "value2",
						},
						Routes: RouteSpec{
							Routes: Routes{
								{Route: "route.example.com"},
								{Route: "another-route.example.com",
									Protocol: HTTP2RouteProtocol,
									Options: RouteOptions{
										LoadBalancing: LeastConnectionLoadBalancingType,
									},
								},
							},
						},
						Services: Services{
							{
								Name: "my-service1",
							},
							{
								Name: "my-service2",
							},
							{
								Name: "my-service-with-arbitrary-params",
								Parameters: map[string]interface{}{
									"key1": "value1",
									"key2": "value2",
								},
								BindingName: "my-service3",
							},
						},
						Stack: "cflinuxfs3",
						ProcessSpecTemplate: &ProcessSpecTemplate{
							Instances: 1,
						},
						Timeout: 120,
						Processes: Processes{
							{
								Type:    Web,
								Timeout: 10,
								ProcessSpecTemplate: ProcessSpecTemplate{
									Command:   "start-web.sh",
									DiskQuota: "512M",
									HealthCheck: ProbeSpec{
										Endpoint: "/healthcheck",
										Timeout:  10,
										Type:     HTTPProbeType,
										Interval: 30,
									},
									ReadinessCheck: ProbeSpec{
										Endpoint: "/",
										Interval: 30,
										Timeout:  1,
										Type:     ProcessProbeType,
									},
									Instances:    3,
									Memory:       "500M",
									LogRateLimit: "16K",
								},
							},
							{
								Type:    Worker,
								Timeout: 15,
								ProcessSpecTemplate: ProcessSpecTemplate{
									Command:   "start-worker.sh",
									DiskQuota: "1G",
									Instances: 2,
									Memory:    "256M",
									HealthCheck: ProbeSpec{
										Type:     ProcessProbeType,
										Endpoint: "/",
										Interval: 30,
										Timeout:  1,
									},
									ReadinessCheck: ProbeSpec{
										Endpoint: "/",
										Timeout:  1,
										Interval: 30,
										Type:     ProcessProbeType,
									},
									LogRateLimit: "16K",
								},
							},
						},
						Sidecars: Sidecars{
							{
								Name:         "authenticator",
								ProcessTypes: []ProcessType{Web, Worker},
								Command:      "bundle exec run-authenticator",
								Memory:       800, // Memory is stored as an int representing MB
							},
							{
								Name:         "upcaser",
								ProcessTypes: []ProcessType{Worker},
								Command:      "./tr-server",
								Memory:       900, // Memory is stored as an int representing MB
							},
						},
					}
					processManifestPath := filepath.Join("test_data", "complete-manifest", "manifest.yml")
					app, err := provider.discoverFromManifestFile(processManifestPath)
					Expect(err).NotTo(HaveOccurred())
					Expect(app).To(BeEquivalentTo(&expected))
				})
			})
		})
		Context("when manifest path is a directory", func() {
			var (
				provider     *CloudFoundryProvider
				manifestPath string
				nopLogger    = logr.New(logr.Discard().GetSink())
				err          error
			)
			BeforeEach(func() {
				manifestPath = filepath.Join("test_data", "multiple_manifests")
				provider, err = New(&Config{ManifestPath: manifestPath}, &nopLogger, true)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns the app name from the single manifest file", func() {
				input := AppReference{
					AppName: "app3",
				}
				apps, err := provider.Discover(input)
				Expect(err).NotTo(HaveOccurred())
				Expect(apps).ToNot(BeNil())
				var resultApp Application
				err = MapToStruct(apps.Content, &resultApp)
				Expect(err).ToNot(HaveOccurred())
				Expect(resultApp).ToNot(Equal(Application{}))
				Expect(resultApp.Metadata).ToNot(Equal(Metadata{}))
				Expect(resultApp.Metadata.Name).To(Equal("app3"))
			})
			It("returns an error if the app name doesn't exists", func() {
				input := AppReference{
					AppName: "not-exists",
				}
				apps, err := provider.Discover(input)
				Expect(err).To(HaveOccurred())
				Expect(apps).To(BeNil())
			})
			It("returns an error if the app name is empty", func() {
				input := AppReference{
					AppName: "",
				}
				apps, err := provider.Discover(input)
				Expect(err).To(HaveOccurred())
				Expect(apps).To(BeNil())
			})
		})
	})

	It("validates no sensitive information is concealed when the provider is configured with the coceal flag to false", func() {
		app := Application{
			Docker: Docker{Username: "username"},
			Services: Services{
				{
					Name:        "elephantsql",
					BindingName: "elephantsql-binding-c6c60",
					Parameters: map[string]interface{}{
						"credentials": `"uri": "postgres://exampleuser:examplepass@babar.elephantsql.com:5432/exampleuser"`, // notsecret
					},
				},
			}}
		nopLogger := logr.New(logr.Discard().GetSink())
		By("Copying the application manifest to be able to check against the resulting changes")
		// copy the app manifest
		b, err := yaml.Marshal(app)
		Expect(err).NotTo(HaveOccurred())
		appCopy := Application{}
		err = yaml.Unmarshal(b, &appCopy)
		Expect(err).NotTo(HaveOccurred())
		provider, err := New(&Config{}, &nopLogger, false)
		Expect(err).NotTo(HaveOccurred())
		By("performing the extraction and modification of the application")
		s := provider.extractSensitiveInformation(&app)
		By("Validating that the app manifest has not been modified")
		Expect(s).To(BeEmpty())
		Expect(appCopy).To(BeEquivalentTo(app))
	})
	DescribeTable("extracts the sensitive information from an app when concealing is expected", func(app Application) {
		By("Copying the application manifest to be able to check against the resulting changes")
		nopLogger := logr.New(logr.Discard().GetSink())
		// copy the app manifest
		b, err := yaml.Marshal(app)
		Expect(err).NotTo(HaveOccurred())
		appCopy := Application{}
		err = yaml.Unmarshal(b, &appCopy)
		Expect(err).NotTo(HaveOccurred())
		provider, err := New(&Config{}, &nopLogger, true)
		Expect(err).NotTo(HaveOccurred())
		By("performing the extraction and modification of the application to use UUID for sensitive information")
		s := provider.extractSensitiveInformation(&app)
		c := 0
		if app.Docker.Username != "" {
			c++
		}
		By("Validating the results")
		for _, s := range app.Services {
			if _, ok := s.Parameters["credentials"]; ok {
				c++
			}
		}
		Expect(s).To(HaveLen(c))
		for k := range app.Env {
			sid := app.Env[k]
			sid = sid[2 : len(sid)-1]
			Expect(s[sid]).To(Equal(appCopy.Env[k]))
		}
		if app.Docker.Username != "" {
			suser := app.Docker.Username[2 : len(app.Docker.Username)-1]
			Expect(s[suser]).To(Equal(appCopy.Docker.Username))
		}

	}, Entry("with docker username and one service with a secret stored in the parameter's map",
		Application{
			Docker: Docker{Username: "username"},
			Services: Services{
				{
					Name:        "elephantsql",
					BindingName: "elephantsql-binding-c6c60",
					Parameters: map[string]interface{}{
						"credentials": `"uri": "postgres://exampleuser:examplepass@babar.elephantsql.com:5432/exampleuser"`, // notsecret
					},
				},
			}}),
		Entry("with docker username and one secret with no credentials stored in the parameter's map",
			Application{
				Docker: Docker{Username: "username"},
				Services: Services{
					{
						Name:        "elephantsql",
						BindingName: "elephantsql-binding-c6c60",
					},
				}}),
		Entry("with no docker username and no environment values",
			Application{
				Docker:   Docker{},
				Services: Services{},
			}),
		Entry("with no docker username but with a service containing a credentials as paramter",
			Application{
				Docker: Docker{Image: "docker.io/library/golang "},
				Services: Services{
					{
						Name:        "sendgrid",
						BindingName: "mysendgrid",
						Parameters: map[string]interface{}{
							"credentials": `{"hostname": "smtp.sendgrid.net","username": "QvsXMbJ3rK","password": "HCHMOYluTv"}`,
						},
					},
				},
			}),
	)

})

func MapToStruct(m map[string]any, obj *Application) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, obj)
}
