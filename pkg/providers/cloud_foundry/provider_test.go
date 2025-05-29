package cloud_foundry

import (
	"io"
	"log"
	"os"

	pTypes "github.com/konveyor/asset-generation/pkg/providers/types/provider"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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

	Context("Type() method", func() {
		It("should return ProviderTypeCF", func() {
			Expect(cfg.Type()).To(Equal(pTypes.ProviderTypeCloudFoundry))
		})
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

})
