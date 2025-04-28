package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	kModels "github.com/konveyor/asset-generation/pkg/discover/cloud_foundry/korifi/models"
	"github.com/pkg/errors"
)

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

func (c *CFAPIClient) ListSpaces() (*kModels.ListResponse[kModels.SpaceResponse], error) {
	resp, err := c.httpClient.Get(c.baseURL + "/v3/spaces")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, resp.Status)
	}

	var i kModels.ListResponse[kModels.SpaceResponse]
	err = json.NewDecoder(resp.Body).Decode(&i)
	if err != nil {
		return nil, errors.Wrap(err, "Error unmarshalling info")
	}
	return &i, nil
}

func (c *CFAPIClient) GetSpace(spaceName string) (*kModels.SpaceResponse, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/v3/spaces?names=" + spaceName)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, resp.Status)
	}

	var i kModels.SpaceResponse
	err = json.NewDecoder(resp.Body).Decode(&i)
	if err != nil {
		return nil, errors.Wrap(err, "Error unmarshalling info")
	}
	return &i, nil
}

func (c *CFAPIClient) ListApps(space string) (*kModels.ListResponse[kModels.AppResponse], error) {
	resp, err := c.httpClient.Get(c.baseURL + "/v3/apps?space_guids=" + space)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, resp.Status)
	}

	var i kModels.ListResponse[kModels.AppResponse]
	err = json.NewDecoder(resp.Body).Decode(&i)
	if err != nil {
		return nil, errors.Wrap(err, "Error unmarshalling info")
	}
	return &i, nil
}

func (c *CFAPIClient) GetEnv(guid string) (*kModels.AppEnvResponse, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/v3/apps/" + guid + "/env")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, resp.Status)
	}

	var i kModels.AppEnvResponse
	err = json.NewDecoder(resp.Body).Decode(&i)
	if err != nil {
		return nil, errors.Wrap(err, "Error unmarshalling info")
	}
	return &i, nil
}
func (c *CFAPIClient) GetProcesses(guid string) (*kModels.ListResponse[kModels.ProcessResponse], error) {
	resp, err := c.httpClient.Get(c.baseURL + "/v3/apps/" + guid + "/processes")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, resp.Status)
	}

	var i kModels.ListResponse[kModels.ProcessResponse]
	err = json.NewDecoder(resp.Body).Decode(&i)
	if err != nil {
		return nil, errors.Wrap(err, "Error unmarshalling info")
	}
	return &i, nil
}

func (c *CFAPIClient) GetRoutes(guid string) (*kModels.ListResponse[kModels.RouteResponse], error) {
	resp, err := c.httpClient.Get(c.baseURL + "/v3/apps/" + guid + "/routes")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, resp.Status)
	}

	var i kModels.ListResponse[kModels.RouteResponse]
	err = json.NewDecoder(resp.Body).Decode(&i)
	if err != nil {
		return nil, errors.Wrap(err, "Error unmarshalling info")
	}
	return &i, nil
}
