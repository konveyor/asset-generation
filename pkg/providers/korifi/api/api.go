package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	kModel "code.cloudfoundry.org/korifi/api/presenter"
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

func (c *KorifiAPIClient) ListSpaces() (*kModel.ListResponse[kModel.SpaceResponse], error) {
	resp, err := c.httpClient.Get(c.baseURL + "/v3/spaces")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, resp.Status)
	}

	var i kModel.ListResponse[kModel.SpaceResponse]
	err = json.NewDecoder(resp.Body).Decode(&i)
	if err != nil {
		return nil, errors.Wrap(err, "Error unmarshalling info")
	}
	return &i, nil
}

func (c *KorifiAPIClient) GetSpace(spaceName string) (*kModel.SpaceResponse, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/v3/spaces?names=" + spaceName)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, resp.Status)
	}

	var i kModel.SpaceResponse
	err = json.NewDecoder(resp.Body).Decode(&i)
	if err != nil {
		return nil, errors.Wrap(err, "Error unmarshalling info")
	}
	return &i, nil
}

func (c *KorifiAPIClient) ListApps(space string) (*kModel.ListResponse[kModel.AppResponse], error) {
	resp, err := c.httpClient.Get(c.baseURL + "/v3/apps?space_guids=\"" + space + "\"")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, resp.Status)
	}

	var i kModel.ListResponse[kModel.AppResponse]
	err = json.NewDecoder(resp.Body).Decode(&i)
	if err != nil {
		return nil, errors.Wrap(err, "Error unmarshalling info")
	}
	return &i, nil
}

func (c *KorifiAPIClient) GetEnv(guid string) (*kModel.AppEnvResponse, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/v3/apps/" + guid + "/env")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, resp.Status)
	}

	var i kModel.AppEnvResponse
	err = json.NewDecoder(resp.Body).Decode(&i)
	if err != nil {
		return nil, errors.Wrap(err, "Error unmarshalling info")
	}
	return &i, nil
}
func (c *KorifiAPIClient) GetProcesses(guid string) (*kModel.ListResponse[kModel.ProcessResponse], error) {
	resp, err := c.httpClient.Get(c.baseURL + "/v3/apps/" + guid + "/processes")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, resp.Status)
	}

	var i kModel.ListResponse[kModel.ProcessResponse]
	err = json.NewDecoder(resp.Body).Decode(&i)
	if err != nil {
		return nil, errors.Wrap(err, "Error unmarshalling info")
	}
	return &i, nil
}

func (c *KorifiAPIClient) GetRoutes(guid string) (*kModel.ListResponse[kModel.RouteResponse], error) {
	resp, err := c.httpClient.Get(c.baseURL + "/v3/apps/" + guid + "/routes")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, resp.Status)
	}

	var i kModel.ListResponse[kModel.RouteResponse]
	err = json.NewDecoder(resp.Body).Decode(&i)
	if err != nil {
		return nil, errors.Wrap(err, "Error unmarshalling info")
	}
	return &i, nil
}
