package korifi

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"strings"

	kHelpers "github.com/konveyor/asset-generation/internal/helpers/korifi"
	ymlHelpers "github.com/konveyor/asset-generation/internal/helpers/yaml"
	cfTypes "github.com/konveyor/asset-generation/pkg/models"
	korifiApi "github.com/konveyor/asset-generation/pkg/providers/korifi/api"
	. "github.com/konveyor/asset-generation/pkg/providers/types"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

type Config struct {
	BaseURL        string
	Username       string
	KubeconfigPath string
	providerType   ProviderType
}

type KorifiProvider struct {
	cfg *Config
}

func New(cfg *Config) *KorifiProvider {
	return &KorifiProvider{cfg: cfg}
}

func (c *Config) Type() ProviderType {
	return c.providerType
}

func (c *KorifiProvider) GetProviderType() ProviderType {
	return ProviderTypeKorifi
}

func (k *KorifiProvider) GetKubeConfig() (*api.Config, error) {
	config, err := clientcmd.LoadFromFile(k.cfg.KubeconfigPath)
	if err != nil {
		fmt.Printf("Error loading kubeconfig: %v\n", err)
		return nil, err
	}
	return config, nil
}

func (k *KorifiProvider) GetKorifiConfig() *Config {
	return k.cfg
}

func (k *KorifiProvider) GetClientCertificate(config *api.Config) (string, error) {
	var dataCert, keyCert []byte
	for authInfoUsername, authInfo := range config.AuthInfos {
		if authInfoUsername == k.cfg.Username {
			dataCert = authInfo.ClientCertificateData
			keyCert = authInfo.ClientKeyData
			break
		}
	}

	if len(dataCert) == 0 || len(keyCert) == 0 {
		return "", fmt.Errorf("could not find certificate data for kind-korifi")
	}

	return base64.StdEncoding.EncodeToString(append(dataCert, keyCert...)), nil
}

func (k *KorifiProvider) GetKorifiHttpClient() (*http.Client, error) {
	k8sConfig, err := k.GetKubeConfig()
	if err != nil {
		return nil, err
	}
	certPEM, err := k.GetClientCertificate(k8sConfig)
	if err != nil {
		return nil, err
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	// Create a custom RoundTripper that adds the Authorization header
	roundTripper := &authHeaderRoundTripper{
		certPEM: certPEM,
		base:    transport,
	}

	// Create an HTTP client with the custom RoundTripper
	return &http.Client{
		Transport: roundTripper,
	}, nil
}

// Custom RoundTripper to add Authorization header
type authHeaderRoundTripper struct {
	certPEM string
	base    http.RoundTripper
}

func (t *authHeaderRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	reqClone := req.Clone(req.Context())

	// Set the Authorization header
	reqClone.Header.Set("Authorization", "ClientCert "+t.certPEM)
	reqClone.Header.Set("X-Username", "kubernetes-admin")
	// Use the base transport to execute the request
	return t.base.RoundTrip(reqClone)
}

func (c *KorifiProvider) OffilineDiscover() ([]Application, error) {
	return nil, fmt.Errorf("not implemented")
}

func (c *KorifiProvider) LiveDiscover(spaceNames []string) error {
	if spaceNames == nil || len(spaceNames) == 0 {
		return fmt.Errorf("no spaces provided for discovery")
	}

	for _, spaceName := range spaceNames {
		log.Println("Analyzing space: ", spaceName)

		korifiHttpClient, err := c.GetKorifiHttpClient()
		if err != nil {
			return fmt.Errorf("error creating Korifi HTTP client: %v", err)
		}
		kAPI := korifiApi.NewKorifiAPIClient(korifiHttpClient, c.cfg.BaseURL)
		// Get space guid
		spaceObj, err := kAPI.GetSpace(spaceName)
		if err != nil {
			return fmt.Errorf("can't find space %s: %v", spaceName, err)
		}
		apps, err := kAPI.ListApps(spaceObj.GUID)
		if err != nil {
			return fmt.Errorf("error listing CF apps for space %s: %v", spaceName, err)
		}

		log.Println("Apps discovered: ", apps.PaginationData.TotalResults)

		for _, app := range apps.Resources {
			log.Println("Processing app:", app.GUID)

			appEnv, err := kAPI.GetEnv(app.GUID)
			if err != nil {
				return fmt.Errorf("error getting environment for app %s: %v", app.GUID, err)
			}

			appName, err := kHelpers.GetAppName(*appEnv)
			if err != nil {
				return fmt.Errorf("error getting app name: %v", err)
			}

			normalizedAppName, err := kHelpers.NormalizeForMetadataName(strings.TrimSpace(appName))
			if err != nil {
				return fmt.Errorf("error normalizing app name: %v", err)
			}

			process, err := kAPI.GetProcesses(app.GUID)
			if err != nil {
				return fmt.Errorf("error getting processes: %v", err)
			}

			appProcesses := cfTypes.AppManifestProcesses{}
			for _, proc := range process.Resources {
				procInstances := uint(proc.Instances)

				appProcesses = append(appProcesses, cfTypes.AppManifestProcess{
					Type:                         cfTypes.AppProcessType(proc.Type),
					Command:                      proc.Command,
					DiskQuota:                    fmt.Sprintf("%d", proc.DiskQuotaMB),
					HealthCheckType:              cfTypes.AppHealthCheckType(proc.HealthCheck.Type),
					HealthCheckHTTPEndpoint:      proc.HealthCheck.Data.HTTPEndpoint,
					HealthCheckInvocationTimeout: uint(proc.HealthCheck.Data.InvocationTimeout),
					Instances:                    &procInstances,
					// LogRateLimitPerSecond
					Memory: fmt.Sprintf("%dMB", proc.MemoryMB),
					// Timeout
					// ReadinessHealthCheckType
					// ReadinessHealthCheckHttpEndpoint
					// ReadinessHealthInvocationTimeout
					// ReadinessHealthCheckInterval
					Lifecycle: string(app.Lifecycle.Type),
				})
			}

			routes, err := kAPI.GetRoutes(app.GUID)
			if err != nil {
				return fmt.Errorf("error getting processes: %v", err)
			}
			appRoutes := cfTypes.AppManifestRoutes{}
			for _, r := range routes.Resources {
				appRoutes = append(appRoutes, cfTypes.AppManifestRoute{
					Route:    r.URL,
					Protocol: cfTypes.AppRouteProtocol(r.Protocol),
					// TODO: Options: loadbalancing?
				})
			}

			labels := kHelpers.ConvertMapToPointer(app.Metadata.Labels)
			annotations := kHelpers.ConvertMapToPointer(app.Metadata.Annotations)
			appManifest := cfTypes.AppManifest{
				Name: normalizedAppName,
				Env:  appEnv.EnvironmentVariables,
				Metadata: &cfTypes.AppMetadata{
					Labels:      labels,
					Annotations: annotations,
				},
				Processes: &appProcesses,
				Routes:    &appRoutes,
				// AppManifestProcess
				// Buildpacks
				// RandomRoute
				// NoRoute
				// Services
				// Sidecars
				// Stack
			}
			if ymlHelpers.WriteToYAMLFile(appManifest, fmt.Sprintf("manifest_%s_%s.yaml", spaceName, appManifest.Name)) != nil {
				return fmt.Errorf("error writing manifest to file: %v", err)
			}
		}
	}
	return nil
}
