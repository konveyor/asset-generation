package provider

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"strings"

	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

// type KorifiProvider interface {
// 	GetProviderType() string
// 	GetKubeConfig() (*api.Config, error)
// 	GetKorifiConfig() *KorifiConfig
// 	GetClientCertificate(config *api.Config) (string, error)
// 	GetKorifiHttpClient() (*http.Client, error)
// }

type KorifiConfig struct {
	BaseURL        string
	Username       string
	KubeconfigPath string
}

type KorifiProvider struct {
	config KorifiConfig
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

func NewKorifiProvider(config KorifiConfig) *KorifiProvider {
	return &KorifiProvider{config: config}
}

func (k *KorifiProvider) GetKubeConfig() (*api.Config, error) {
	return getKubeConfig(k.config.KubeconfigPath)
}

func (k *KorifiProvider) GetProviderType() ProviderType {
	return ProviderTypeKorifi
}

func (k *KorifiProvider) GetKorifiConfig() *KorifiConfig {
	return &k.config
}

func (k *KorifiProvider) GetClientCertificate(config *api.Config) (string, error) {
	return getClientCertificate(config, k.config.Username)
}

func (k *KorifiProvider) GetClient() (interface{}, error) {
	return getKorifiHttpClient(k.config.KubeconfigPath, k.config.Username)
}

func getKubeConfig(kubeconfigPath string) (*api.Config, error) {
	config, err := clientcmd.LoadFromFile(kubeconfigPath)
	if err != nil {
		fmt.Printf("Error loading kubeconfig: %v\n", err)
		return nil, err
	}
	return config, nil
}

func getClientCertificate(config *api.Config, username string) (string, error) {
	var dataCert, keyCert []byte
	for authInfoUsername, authInfo := range config.AuthInfos {
		if authInfoUsername == username {
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

func getKorifiHttpClient(kubeconfigPath string, username string) (*http.Client, error) {
	config, err := getKubeConfig(kubeconfigPath)
	if err != nil {
		return nil, err
	}
	certPEM, err := getClientCertificate(config, username)
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

func (c *KorifiProvider) Discover(spaceName string) error {
	var spaceNames []string
	if ld.spaceNames == nil || len(*ld.spaceNames) == 0 {
		return fmt.Errorf("no spaces provided for discovery")
	}

	spaceNames = *ld.spaceNames
	for _, spaceName := range spaceNames {
		log.Println("Analyzing space: ", spaceName)

		// Get space guid
		spaceObj, err := ld.cfAPI.GetSpace(spaceName)
		if err != nil {
			return fmt.Errorf("can't find space %s: %v", spaceName, err)
		}
		apps, err := ld.cfAPI.ListApps(spaceObj.GUID)
		if err != nil {
			return fmt.Errorf("error listing CF apps for space %s: %v", spaceName, err)
		}

		log.Println("Apps discovered: ", apps.PaginationData.TotalResults)

		for _, app := range apps.Resources {
			log.Println("Processing app:", app.GUID)

			appEnv, err := ld.cfAPI.GetEnv(app.GUID)
			if err != nil {
				return fmt.Errorf("error getting environment for app %s: %v", app.GUID, err)
			}

			appName, err := kApi.GetAppName(*appEnv)
			if err != nil {
				return fmt.Errorf("error getting app name: %v", err)
			}

			normalizedAppName, err := kApi.NormalizeForMetadataName(strings.TrimSpace(appName))
			if err != nil {
				return fmt.Errorf("error normalizing app name: %v", err)
			}

			process, err := ld.cfAPI.GetProcesses(app.GUID)
			if err != nil {
				return fmt.Errorf("error getting processes: %v", err)
			}

			appProcesses := AppManifestProcesses{}
			for _, proc := range process.Resources {
				procInstances := uint(proc.Instances)

				appProcesses = append(appProcesses, AppManifestProcess{
					Type:                         AppProcessType(proc.Type),
					Command:                      proc.Command,
					DiskQuota:                    fmt.Sprintf("%d", proc.DiskQuotaMB),
					HealthCheckType:              AppHealthCheckType(proc.HealthCheck.Type),
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

			routes, err := ld.cfAPI.GetRoutes(app.GUID)
			if err != nil {
				return fmt.Errorf("error getting processes: %v", err)
			}
			appRoutes := AppManifestRoutes{}
			for _, r := range routes.Resources {
				appRoutes = append(appRoutes, AppManifestRoute{
					Route:    r.URL,
					Protocol: AppRouteProtocol(r.Protocol),
					// TODO: Options: loadbalancing?
				})
			}

			labels := kApi.ConvertMapToPointer(app.Metadata.Labels)
			annotations := kApi.ConvertMapToPointer(app.Metadata.Annotations)
			appManifest := AppManifest{
				Name: normalizedAppName,
				Env:  appEnv.EnvironmentVariables,
				Metadata: &AppMetadata{
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
			if writeToYAMLFile(appManifest, fmt.Sprintf("manifest_%s_%s.yaml", spaceName, appManifest.Name)) != nil {
				return fmt.Errorf("error writing manifest to file: %v", err)
			}
		}
	}
	return nil
}
