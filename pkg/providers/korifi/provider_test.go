package korifi

import (
	"crypto/tls"
	"encoding/base64"
	"io"
	"log"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/tools/clientcmd/api"
)

var _ = Describe("KubeConfig Utilities", func() {
	logger := log.New(io.Discard, "", log.LstdFlags) // No-op logger

	Describe("getKubeConfig", func() {
		It("returns error when file does not exist", func() {
			provider := New(
				&Config{}, logger)

			config, err := provider.GetKubeConfig()
			Expect(config).To(BeNil())
			Expect(err).To(HaveOccurred())
		})

		It("returns config when file is valid", func() {
			provider := New(
				&Config{
					KubeconfigPath: "./test_data/test_config.yaml",
				},
				logger)
			config, err := provider.GetKubeConfig()
			Expect(config).ToNot(BeNil())
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("getClientCertificate", func() {
		var config *api.Config

		BeforeEach(func() {
			config = &api.Config{
				AuthInfos: map[string]*api.AuthInfo{
					"kind-korifi": {
						ClientCertificateData: []byte("cert"),
						ClientKeyData:         []byte("key"),
					},
				},
			}
		})

		It("returns base64 encoded cert+key for existing user", func() {
			provider := New(
				&Config{
					KubeconfigPath: "./test_data/test_config.yaml",
					Username:       "kind-korifi",
				},
				logger,
			)
			cert, err := provider.GetClientCertificate(config)
			Expect(err).NotTo(HaveOccurred())
			expected := base64.StdEncoding.EncodeToString([]byte("certkey"))
			Expect(cert).To(Equal(expected))
		})

		It("returns error if user does not exist", func() {
			provider := New(
				&Config{
					KubeconfigPath: "./test_data/test_config.yaml",
					Username:       "non-existent-user",
				},
				logger,
			)
			cert, err := provider.GetClientCertificate(config)
			Expect(cert).To(BeEmpty())
			Expect(err).To(MatchError(ContainSubstring("could not find certificate data")))
		})

		It("returns error if cert or key data missing", func() {
			provider := New(
				&Config{
					KubeconfigPath: "./test_data/test_config.yaml",
					Username:       "kind-korifi",
				},
				logger,
			)
			config.AuthInfos["kind-korifi"].ClientCertificateData = nil
			cert, err := provider.GetClientCertificate(config)
			Expect(cert).To(BeEmpty())
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("getKorifiHttpClient", func() {
		It("returns error if kubeconfig is invalid", func() {
			provider := New(
				&Config{
					KubeconfigPath: "./test_data/non-existing-config.yaml",
				},
				logger,
			)
			client, err := provider.GetKorifiHttpClient()
			Expect(client).To(BeNil())
			Expect(err).To(HaveOccurred())
		})

		It("returns error if user not found in kubeconfig", func() {
			provider := New(
				&Config{
					KubeconfigPath: "./test_data/test_config.yaml",
					Username:       "non-existent-user",
				},
				logger,
			)
			client, err := provider.GetKorifiHttpClient()
			Expect(client).To(BeNil())
			Expect(err).To(HaveOccurred())
		})

		It("returns http.Client when everything is valid", func() {
			provider := New(
				&Config{
					KubeconfigPath: "./test_data/test_config.yaml",
					Username:       "kind-korifi",
				},
				logger,
			)
			client, err := provider.GetKorifiHttpClient()
			Expect(err).NotTo(HaveOccurred())
			Expect(client).NotTo(BeNil())
			Expect(client.Transport).NotTo(BeNil())
			Expect(client.Transport).NotTo(BeNil())

			apiConfig, err := provider.GetKubeConfig()
			Expect(err).NotTo(HaveOccurred())

			cert, _ := provider.GetClientCertificate(apiConfig)
			transport := &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			}
			roundTripper := &authHeaderRoundTripper{
				certPEM: cert,
				base:    transport,
			}
			Expect(client.Transport).To(Equal(roundTripper))
		})
	})
})
