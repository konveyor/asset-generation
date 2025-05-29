package cloud_foundry

import (
	"io"
	"log"
	"os"

	dTypes "github.com/konveyor/asset-generation/pkg/providers/types/discover"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
)

var _ = Describe("CFProvider", func() {
	var cfg *Config
	var provider *CloudFoundryProvider
	logger := log.New(io.Discard, "", log.LstdFlags) // No-op logger
	BeforeEach(func() {
		cfg = &Config{
			CloudFoundryConfigPath: "/some/path/to/config.json",
			Username:               "admin",
			Password:               "password",
			APIEndpoint:            "https://api.example.com",
		}
		provider = New(cfg, logger)
	})

	Context("GetClient", func() {
		It("should fail if CF_HOME config is invalid", func() {
			CFHomeOrig, had := os.LookupEnv("CF_HOME")
			os.Setenv("CF_HOME", "/non/existent/dir")
			defer func() {
				if had {
					os.Setenv("CF_HOME", CFHomeOrig)
				} else {
					os.Unsetenv("CF_HOME")
				}
			}()

			client, err := provider.GetClient()
			Expect(client).To(BeNil())
			Expect(err).To(HaveOccurred())
		})
		It("should create client successfully", func() {
			CFHomeOrig, had := os.LookupEnv("CF_HOME")
			os.Setenv("CF_HOME", "./test_data")
			defer func() {
				if had {
					os.Setenv("CF_HOME", CFHomeOrig)
				} else {
					os.Unsetenv("CF_HOME")
				}
			}()

			client, err := provider.GetClient()
			Expect(err).ToNot(HaveOccurred())
			Expect(client).NotTo(BeNil())
		})

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
