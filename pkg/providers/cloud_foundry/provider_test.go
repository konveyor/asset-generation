package cloud_foundry

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/config"
	"github.com/cloudfoundry/go-cfclient/v3/testutil"
	getter "github.com/hashicorp/go-getter"
	dTypes "github.com/konveyor/asset-generation/pkg/providers/types/discover"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
)

const (
	goCFClientTemplateURL = "git::https://github.com/cloudfoundry/go-cfclient.git//testutil/template"
)

var _ = Describe("CloudFoundry Provider", func() {
	var (
		// provider *CloudFoundryProvider
		logger *log.Logger
		// testServer *testutil.SetupFakeAPIServer
		// mockClient *client.Client

	)

	BeforeEach(func() {
		logger = log.New(io.Discard, "", log.LstdFlags) // no-op logger
	})

	Describe("GetClient", func() {
		var (
			origCFHome string
			hadCFHome  bool
			provider   *CloudFoundryProvider
		)

		BeforeEach(func() {
			origCFHome, hadCFHome = os.LookupEnv("CF_HOME")
			provider = New(&Config{}, logger)
		})

		AfterEach(func() {
			if hadCFHome {
				os.Setenv("CF_HOME", origCFHome)
			} else {
				os.Unsetenv("CF_HOME")
			}
		})

		It("returns error if CF_HOME points to invalid directory", func() {
			err := os.Setenv("CF_HOME", "/non/existent/dir")
			Expect(err).NotTo(HaveOccurred())

			client, err := provider.GetClient()
			Expect(client).To(BeNil())
			Expect(err).To(HaveOccurred())
		})

		It("creates client successfully with valid CF_HOME", func() {
			err := os.Setenv("CF_HOME", "./test_data")
			Expect(err).NotTo(HaveOccurred())

			client, err := provider.GetClient()
			Expect(err).NotTo(HaveOccurred())
			Expect(client).NotTo(BeNil())
		})
	})

	Describe("listAppsFromCloudFoundry", Ordered, func() {
		var (
			g            *testutil.ObjectJSONGenerator
			app1         *testutil.JSONResource
			app2         *testutil.JSONResource
			space        *testutil.JSONResource
			serverURL    string
			logger       = log.New(io.Discard, "", 0)
			templatePath string
		)

		AfterAll(func() {
			os.RemoveAll(templatePath)
			testutil.Teardown()
		})

		BeforeAll(func() {
			repoBasePath := getModuleRoot()
			templatePath = filepath.Join(repoBasePath,
				"vendor", "github.com", "cloudfoundry", "go-cfclient", "v3", "testutil", "template")

			err := downloadTemplateFolder(goCFClientTemplateURL, templatePath)
			if err != nil {
				log.Fatalf("Failed to download template folder: %v", err)
			}

			g = testutil.NewObjectJSONGenerator()
			space = g.Space()
			app1 = g.Application()
			app2 = g.Application()
		})
		Context("when space name doens't exist", func() {
			BeforeEach(func() {
				pagingQueryString := "page=1&per_page=50"
				serverURL = testutil.SetupMultiple([]testutil.MockRoute{
					{
						Method:      "GET",
						Endpoint:    "/v3/spaces",
						Output:      g.Paged([]string{}),
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

				client, err := client.New(cfg)
				Expect(err).NotTo(HaveOccurred())

				cfConfig := &Config{
					Client:    client,
					Username:  "username",
					SpaceName: space.Name,
				}

				p := New(cfConfig, logger)
				apps, err := p.listAppsFromCloudFoundry()
				Expect(err).To(HaveOccurred())
				Expect(apps).To(BeNil())

			})
		})
		Context("when apps exist in the space", func() {
			BeforeEach(func() {
				pagingQueryString := "page=1&per_page=50"
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

			It("returns the GUIDs of the apps", func() {
				cfg, err := config.New(serverURL, config.Token("", "fake-refresh-token"), config.SkipTLSValidation())
				Expect(err).NotTo(HaveOccurred())

				client, err := client.New(cfg)
				Expect(err).NotTo(HaveOccurred())

				cfConfig := &Config{
					Client:    client,
					Username:  "username",
					SpaceName: space.Name,
				}

				p := New(cfConfig, logger)
				apps, err := p.listAppsFromCloudFoundry()
				Expect(err).NotTo(HaveOccurred())
				Expect(apps).To(HaveLen(2))
				Expect(apps).To(ConsistOf(app1.GUID, app2.GUID))
			})
		})
		Context("when apps don't exist in the space", func() {
			BeforeEach(func() {
				// Create two mock apps in the test server
				pagingQueryString := "page=1&per_page=50"
				serverURL = testutil.SetupMultiple([]testutil.MockRoute{
					{
						Method:      "GET",
						Endpoint:    "/v3/apps",
						Output:      g.Paged([]string{}),
						Status:      http.StatusOK,
						QueryString: pagingQueryString + "&space_guids=" + space.GUID,
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

			It("returns no apps", func() {
				cfg, err := config.New(serverURL, config.Token("", "fake-refresh-token"), config.SkipTLSValidation())
				Expect(err).NotTo(HaveOccurred())

				client, err := client.New(cfg)
				Expect(err).NotTo(HaveOccurred())

				cfConfig := &Config{
					Client:    client,
					Username:  "username",
					SpaceName: space.Name,
				}

				p := New(cfConfig, logger)
				apps, err := p.listAppsFromCloudFoundry()
				Expect(err).NotTo(HaveOccurred())
				Expect(apps).To(HaveLen(0))
			})
		})

		// 	Context("when the CF API returns an error", func() {
		// 		BeforeEach(func() {
		// 			testServer.ForceAPIErrors()
		// 		})

		// 		It("returns a wrapped error", func() {
		// 			_, err := provider.listAppsFromCloudFoundry()
		// 			Expect(err).To(MatchError(ContainSubstring("error listing Cloud Foundry apps")))
		// 		})
		// 	})

		// 	Context("when filtering by space name", func() {
		// 		It("returns only apps in the configured space", func() {
		// 			// Create apps in different spaces
		// 			testServer.Resources().Applications().Create(
		// 				testutil.ApplicationResource().WithName("app-in-test-space").WithGUID("guid-test").WithSpace("test-space"),
		// 				testutil.ApplicationResource().WithName("app-in-other-space").WithGUID("guid-other").WithSpace("other-space"),
		// 			)

		// 			guids, err := provider.listAppsFromCloudFoundry()
		// 			Expect(err).NotTo(HaveOccurred())
		// 			Expect(guids).To(ConsistOf("guid-test"))
		// 		})
		// 	})
	})

	DescribeTable("extracts the sensitive infromation from an app", func(app dTypes.Application) {
		By("Copying the application manifest to be able to check against the resulting changes")
		// copy the app manifest
		b, err := yaml.Marshal(app)
		Expect(err).NotTo(HaveOccurred())
		appCopy := dTypes.Application{}
		err = yaml.Unmarshal(b, &appCopy)
		Expect(err).NotTo(HaveOccurred())
		By("performing the extraction and modification of the application to use UUID for sensitive information")
		s := extractSensitiveInformation(&app)
		c := 0
		if app.Docker.Username != "" {
			c++
		}
		By("Validating the results")
		Expect(s).To(HaveLen(c + len(app.Env)))
		for k := range app.Env {
			sid := app.Env[k]
			sid = sid[2 : len(sid)-1]
			Expect(s[sid]).To(Equal(appCopy.Env[k]))
		}
		if app.Docker.Username != "" {
			suser := app.Docker.Username[2 : len(app.Docker.Username)-1]
			Expect(s[suser]).To(Equal(appCopy.Docker.Username))
		}

	}, Entry("with docker username and environment values",
		dTypes.Application{
			Docker: dTypes.Docker{Username: "username"},
			Env:    map[string]string{"RAILS_ENV": "production", "mysql": "[{\"name\": \"db-for-my-app\",\"label\": \"mysql\",\"tags\": [\"relational\", \"sql\"],\"plan\": \"xlarge\",\"credentials\": {\"username\": \"user\",\"password\": \"top-secret\"},\"syslog_drain_url\": \"https://syslog.example.org/drain\",\"provider\": null}]"},
		}),
		Entry("with docker username and no environment values",
			dTypes.Application{
				Docker: dTypes.Docker{Username: "username"},
				Env:    map[string]string{},
			}),
		Entry("with no docker username and no environment values",
			dTypes.Application{
				Docker: dTypes.Docker{},
				Env:    map[string]string{},
			}),
		Entry("with no docker username but with environment values",
			dTypes.Application{
				Docker: dTypes.Docker{Image: "docker.io/library/golang "},
				Env:    map[string]string{"RAILS_ENV": "production", "LOG_LEVEL": "debug", "mysql": "[{\"name\": \"db-for-my-app\",\"label\": \"mysql\",\"tags\": [\"relational\", \"sql\"],\"plan\": \"xlarge\",\"credentials\": {\"username\": \"user\",\"password\": \"top-secret\"},\"syslog_drain_url\": \"https://syslog.example.org/drain\",\"provider\": null}]"},
			}),
	)

})

func downloadTemplateFolder(src string, dst string) error {
	client := &getter.Client{
		Ctx:      context.Background(),
		Src:      src,
		Dst:      dst,
		Dir:      true,
		Mode:     getter.ClientModeDir,
		Insecure: true,
	}

	if err := client.Get(); err != nil {
		return fmt.Errorf("failed to download from %q to %q: %w", src, dst, err)
	}
	return nil
}

func getModuleRoot() string {
	// gomodPath := os.Getenv("GOMOD")
	// if gomodPath == "" {
	// 	log.Fatal("GOMOD environment variable is not set; make sure to run with Go modules enabled")
	// }
	out, err := exec.Command("go", "env", "GOMOD").Output()
	if err != nil {
		log.Fatalf("Failed to get GOMOD via 'go env': %v", err)
	}
	gomodPath := strings.TrimSpace(string(out))
	if gomodPath == "" {
		log.Fatal("GOMOD is empty")
	}
	return filepath.Dir(gomodPath)
}
