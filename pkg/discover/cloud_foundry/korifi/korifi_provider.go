package korifi

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net/http"

	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

// Assisted by watsonx Code Assistant
type KorifiProvider interface {
	GetKubeConfig() (*api.Config, error)
	GetClientCertificate(config *api.Config) (string, error)
	GetKorifiHttpClient() (*http.Client, error)
}

type KorifiConfig struct {
	baseURL        string
	username       string
	kubeconfigPath string
}

type korifiProviderImpl struct {
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

func NewKorifiProvider(config KorifiConfig) KorifiProvider {
	return &korifiProviderImpl{config: config}
}
func (k *korifiProviderImpl) GetKubeConfig() (*api.Config, error) {
	return getKubeConfig(k.config.kubeconfigPath)
}

func (k *korifiProviderImpl) GetClientCertificate(config *api.Config) (string, error) {
	return getClientCertificate(config, k.config.username)
}

func (k *korifiProviderImpl) GetKorifiHttpClient() (*http.Client, error) {
	return getKorifiHttpClient(k.config.kubeconfigPath, k.config.username)
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
	// Find the desired user context (in this case, "kind-korifi")
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
