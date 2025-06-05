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

	"github.com/cloudfoundry/go-cfclient/v3/config"
	"github.com/cloudfoundry/go-cfclient/v3/testutil"
	getter "github.com/hashicorp/go-getter"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
)

const (
	goCFClientTemplateURL = "git::https://github.com/cloudfoundry/go-cfclient.git//testutil/template"
)

var _ = Describe("CloudFoundry Provider", func() {

	Describe("listAppsFromCloudFoundry", Ordered, func() {
		var (
			g            *testutil.ObjectJSONGenerator
			app1         *testutil.JSONResource
			app2         *testutil.JSONResource
			space        *testutil.JSONResource
			emptySpace   *testutil.JSONResource
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
			emptySpace = g.Space()
			app1 = g.Application()
			app2 = g.Application()
		})
		Context("when space name doesn't exist", func() {
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

				cfConfig := &Config{
					SpaceNames:         []string{space.Name},
					CloudFoundryConfig: cfg,
				}

				p, err := New(cfConfig, logger)
				Expect(err).NotTo(HaveOccurred())
				apps, err := p.ListApps()
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

				p, err := New(cfConfig, logger)
				Expect(err).NotTo(HaveOccurred())
				apps, err := p.ListApps()
				Expect(err).NotTo(HaveOccurred())
				Expect(apps).To(HaveLen(1))
				Expect(apps).To(HaveKey(space.Name))
				Expect(apps[space.Name]).To(BeEquivalentTo([]string{app1.GUID, app2.GUID}))
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

				p, err := New(cfConfig, logger)
				Expect(err).NotTo(HaveOccurred())
				apps, err := p.listAppsFromCloudFoundry()
				Expect(err).NotTo(HaveOccurred())
				Expect(apps).To(HaveLen(1))
				Expect(apps[emptySpace.Name]).To(BeEmpty())
			})
		})
	})

	DescribeTable("extracts the sensitive information from an app", func(app Application) {
		By("Copying the application manifest to be able to check against the resulting changes")
		// copy the app manifest
		b, err := yaml.Marshal(app)
		Expect(err).NotTo(HaveOccurred())
		appCopy := Application{}
		err = yaml.Unmarshal(b, &appCopy)
		Expect(err).NotTo(HaveOccurred())
		By("performing the extraction and modification of the application to use UUID for sensitive information")
		s := extractSensitiveInformation(&app)
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
						"credentials": `"uri": "postgres://exampleuser:examplepass@babar.elephantsql.com:5432/exampleuser"`,
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
