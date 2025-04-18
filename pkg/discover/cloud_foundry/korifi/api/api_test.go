package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	korifi "github.com/konveyor/asset-generation/pkg/discover/cloud_foundry/korifi/models"
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
		testSpaceGUID := "example93847523894052745"
		It("should return the list of apps from the API", func() {
			result, err := client.ListApps(testSpaceGUID)

			Expect(err).ToNot(HaveOccurred())
			Expect(result.Resources).To(HaveLen(2))
			Expect(result.Resources[0].GUID).To(Equal("app-1"))
			Expect(result.Resources[0].Name).To(Equal("App One"))
			Expect(result.Resources[1].GUID).To(Equal("app-2"))
			Expect(result.Resources[1].Name).To(Equal("App Two"))
		})
		It("should handle empty response gracefully", func() {
			server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(korifi.ListResponse[korifi.AppResponse]{})
			})

			result, err := client.ListApps(testSpaceGUID)

			Expect(err).ToNot(HaveOccurred())
			Expect(result.Resources).To(BeEmpty())
		})

		It("should handle non-200 status codes gracefully", func() {
			server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("Internal Server Error"))
			})

			result, err := client.ListApps(testSpaceGUID)

			Expect(err).To(HaveOccurred())
			Expect(result).To(BeNil())
			Expect(err.Error()).To(ContainSubstring("request failed with status 500"))
		})
	})

	Describe("GetEnv", func() {
		It("should return the environment variables for a given app", func() {
			appGUID := "app-1"
			mockEnvResponse := korifi.AppEnvResponse{
				EnvironmentVariables: map[string]string{
					"VAR1": "value1",
					"VAR2": "value2",
				},
			}

			server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.URL.Path).To(Equal("/v3/apps/" + appGUID + "/env"))
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(mockEnvResponse)
			})

			result, err := client.GetEnv(appGUID)

			Expect(err).ToNot(HaveOccurred())
			Expect(result.EnvironmentVariables["VAR1"]).To(Equal("value1"))
			Expect(result.EnvironmentVariables["VAR2"]).To(Equal("value2"))
		})
		It("should handle empty environment variables gracefully", func() {
			appGUID := "app-1"
			server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.URL.Path).To(Equal("/v3/apps/" + appGUID + "/env"))
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(korifi.AppEnvResponse{EnvironmentVariables: map[string]string{}})
			})

			result, err := client.GetEnv(appGUID)

			Expect(err).ToNot(HaveOccurred())
			Expect(result.EnvironmentVariables).To(BeEmpty())
		})
		It("should handle non-200 status codes gracefully", func() {
			appGUID := "app-1"
			server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte("Not Found"))
			})

			result, err := client.GetEnv(appGUID)

			Expect(err).To(HaveOccurred())
			Expect(result).To(BeNil())
			Expect(err.Error()).To(ContainSubstring("request failed with status 404"))
		})
		It("should handle JSON unmarshalling errors gracefully", func() {
			appGUID := "app-1"
			server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.URL.Path).To(Equal("/v3/apps/" + appGUID + "/env"))
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("Invalid JSON"))
			})

			result, err := client.GetEnv(appGUID)

			Expect(err).To(HaveOccurred())
			Expect(result).To(BeNil())
			Expect(err.Error()).To(ContainSubstring("Error unmarshalling info"))
		})
		It("should handle network errors gracefully", func() {
			appGUID := "app-1"
			server.Close() // Close the server to simulate a network error

			result, err := client.GetEnv(appGUID)

			Expect(err).To(HaveOccurred())
			Expect(result).To(BeNil())
			Expect(err.Error()).To(ContainSubstring("Get"))
		})
	})
	Describe("GetProcesses", func() {
		It("should return the processes for a given app", func() {
			appGUID := "app-1"
			mockProcessResponse := korifi.ListResponse[korifi.ProcessResponse]{
				Resources: []korifi.ProcessResponse{
					{GUID: "process-1", Type: "web"},
					{GUID: "process-2", Type: "worker"},
				},
			}

			server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.URL.Path).To(Equal("/v3/apps/" + appGUID + "/processes"))
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(mockProcessResponse)
			})

			result, err := client.GetProcesses(appGUID)

			Expect(err).ToNot(HaveOccurred())
			Expect(result.Resources).To(HaveLen(2))
			Expect(result.Resources[0].GUID).To(Equal("process-1"))
			Expect(result.Resources[0].Type).To(Equal("web"))
			Expect(result.Resources[1].GUID).To(Equal("process-2"))
			Expect(result.Resources[1].Type).To(Equal("worker"))
		})
		It("should handle empty processes gracefully", func() {
			appGUID := "app-1"
			server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.URL.Path).To(Equal("/v3/apps/" + appGUID + "/processes"))
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(korifi.ListResponse[korifi.ProcessResponse]{})
			})

			result, err := client.GetProcesses(appGUID)

			Expect(err).ToNot(HaveOccurred())
			Expect(result.Resources).To(BeEmpty())
		})
	})
	Describe("GetRoutes", func() {
		It("should return the routes for a given app", func() {
			appGUID := "app-1"
			mockRouteResponse := korifi.ListResponse[korifi.RouteResponse]{
				Resources: []korifi.RouteResponse{
					{GUID: "route-1", Host: "example.com"},
					{GUID: "route-2", Host: "test.com"},
				},
			}

			server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.URL.Path).To(Equal("/v3/apps/" + appGUID + "/routes"))
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(mockRouteResponse)
			})

			result, err := client.GetRoutes(appGUID)

			Expect(err).ToNot(HaveOccurred())
			Expect(result.Resources).To(HaveLen(2))
			Expect(result.Resources[0].GUID).To(Equal("route-1"))
			Expect(result.Resources[0].Host).To(Equal("example.com"))
			Expect(result.Resources[1].GUID).To(Equal("route-2"))
			Expect(result.Resources[1].Host).To(Equal("test.com"))
		})
		It("should handle empty routes gracefully", func() {
			appGUID := "app-1"
			server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.URL.Path).To(Equal("/v3/apps/" + appGUID + "/routes"))
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(korifi.ListResponse[korifi.RouteResponse]{})
			})

			result, err := client.GetRoutes(appGUID)

			Expect(err).ToNot(HaveOccurred())
			Expect(result.Resources).To(BeEmpty())
		})
	})
})
