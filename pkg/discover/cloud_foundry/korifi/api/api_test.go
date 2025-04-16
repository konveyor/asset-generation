package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	// api "github.com/konveyor/asset-generation/pkg/discover/cloud_foundry/korifi/api"

	"github.com/konveyor/asset-generation/pkg/discover/cloud_foundry/korifi"
)

var _ = Describe("CFAPIClient", func() {
	var (
		server       *httptest.Server
		client       *CFAPIClient
		mockResponse korifi.ListResponse[korifi.AppResponse]
	)

	BeforeEach(func() {
		// Mock server setup
		mockResponse = korifi.ListResponse[korifi.AppResponse]{
			Resources: []korifi.AppResponse{
				{GUID: "app-1", Name: "App One"},
				{GUID: "app-2", Name: "App Two"},
			},
		}

		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			Expect(r.URL.Path).To(Equal("/v3/apps"))
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(mockResponse)
		}))

		client = NewCFAPIClient(&http.Client{}, server.URL)
	})

	AfterEach(func() {
		server.Close()
	})

	Describe("ListApps", func() {
		It("should return the list of apps from the API", func() {
			result, err := client.ListApps()

			Expect(err).ToNot(HaveOccurred())
			Expect(result.Resources).To(HaveLen(2))
			Expect(result.Resources[0].GUID).To(Equal("app-1"))
			Expect(result.Resources[0].Name).To(Equal("App One"))
			Expect(result.Resources[1].GUID).To(Equal("app-2"))
			Expect(result.Resources[1].Name).To(Equal("App Two"))
		})

		It("should handle non-200 status codes gracefully", func() {
			server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("Internal Server Error"))
			})

			result, err := client.ListApps()

			Expect(err).To(HaveOccurred())
			Expect(result).To(BeNil())
			Expect(err.Error()).To(ContainSubstring("request failed with status 500"))
		})
	})
})
