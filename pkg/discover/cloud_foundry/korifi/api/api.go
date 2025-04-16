package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/konveyor/asset-generation/pkg/discover/cloud_foundry/korifi"
	"github.com/pkg/errors"
)

// CF API Client
type CFAPIClient struct {
	httpClient *http.Client
	baseURL    string
}

func NewCFAPIClient(httpClient *http.Client, baseURL string) *CFAPIClient {
	return &CFAPIClient{
		httpClient: httpClient,
		baseURL:    baseURL,
	}
}

func (c *CFAPIClient) ListApps() (*korifi.ListResponse[korifi.AppResponse], error) {
	resp, err := c.httpClient.Get(c.baseURL + "/v3/apps")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, resp.Status)
	}

	var i korifi.ListResponse[korifi.AppResponse]
	err = json.NewDecoder(resp.Body).Decode(&i)
	if err != nil {
		return nil, errors.Wrap(err, "Error unmarshalling info")
	}
	return &i, nil
}

func (c *CFAPIClient) GetEnv(guid string) (*korifi.AppEnvResponse, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/v3/apps/" + guid + "/env")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, resp.Status)
	}

	var i korifi.AppEnvResponse
	err = json.NewDecoder(resp.Body).Decode(&i)
	if err != nil {
		return nil, errors.Wrap(err, "Error unmarshalling info")
	}
	return &i, nil
}
func (c *CFAPIClient) GetProcesses(guid string) (*korifi.ListResponse[korifi.ProcessResponse], error) {
	resp, err := c.httpClient.Get(c.baseURL + "/v3/apps/" + guid + "/processes")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, resp.Status)
	}

	var i korifi.ListResponse[korifi.ProcessResponse]
	err = json.NewDecoder(resp.Body).Decode(&i)
	if err != nil {
		return nil, errors.Wrap(err, "Error unmarshalling info")
	}
	return &i, nil
}

func (c *CFAPIClient) GetRoutes(guid string) (*korifi.ListResponse[korifi.RouteResponse], error) {
	resp, err := c.httpClient.Get(c.baseURL + "/v3/apps/" + guid + "/routes")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, resp.Status)
	}

	var i korifi.ListResponse[korifi.RouteResponse]
	err = json.NewDecoder(resp.Body).Decode(&i)
	if err != nil {
		return nil, errors.Wrap(err, "Error unmarshalling info")
	}
	return &i, nil
}
