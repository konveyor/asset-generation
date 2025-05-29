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

	It("extracts the sensitive information from an app with docker username and environment values", func() {
		app := dTypes.Application{
			Docker: dTypes.Docker{Username: "username"},
			Env:    map[string]string{"a": "a", "b": "b", "c": "c"},
		}
		By("Copying the application manifest to be able to check against the resulting changes")
		// copy the app manifest
		b, err := yaml.Marshal(app)
		Expect(err).NotTo(HaveOccurred())
		appCopy := dTypes.Application{}
		err = yaml.Unmarshal(b, &appCopy)
		Expect(err).NotTo(HaveOccurred())
		By("performing the extraction and modification of the application to use UUID for sensitive information")
		s := extractSensitiveInformation(&app)
		Expect(s).To(HaveLen(4))
		for k := range app.Env {
			sid := app.Env[k]
			sid = sid[2 : len(sid)-1]
			Expect(s[sid]).To(Equal(appCopy.Env[k]))
		}
		suser := app.Docker.Username[2 : len(app.Docker.Username)-1]
		Expect(s[suser]).To(Equal(appCopy.Docker.Username))
	})

})
