package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	kModels "github.com/konveyor/asset-generation/pkg/providers/korifi/models"
	"github.com/pkg/errors"
)

type KorifiAPIClient struct {
	httpClient *http.Client
	baseURL    string
}

func NewKorifiAPIClient(httpClient *http.Client, baseURL string) *KorifiAPIClient {
	return &KorifiAPIClient{
		httpClient: httpClient,
		baseURL:    baseURL,
	}
}

func (c *KorifiAPIClient) ListSpaces() (*kModels.ListResponse[kModels.SpaceResponse], error) {
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

func (c *KorifiAPIClient) GetSpace(spaceName string) (*kModels.SpaceResponse, error) {
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

func (c *KorifiAPIClient) ListApps(space string) (*kModels.ListResponse[kModels.AppResponse], error) {
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

func (c *KorifiAPIClient) GetEnv(guid string) (*kModels.AppEnvResponse, error) {
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
func (c *KorifiAPIClient) GetProcesses(guid string) (*kModels.ListResponse[kModels.ProcessResponse], error) {
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

func (c *KorifiAPIClient) GetRoutes(guid string) (*kModels.ListResponse[kModels.RouteResponse], error) {
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
