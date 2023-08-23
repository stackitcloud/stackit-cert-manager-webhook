package resolver

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	extapi "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

//go:generate mockgen -destination=./mock/config.go -source=./config.go ConfigProvider
type ConfigProvider interface {
	LoadConfig(cfgJSON *extapi.JSON) (StackitDnsProviderConfig, error)
}

type defaultConfigProvider struct{}

type StackitDnsProviderConfig struct {
	ProjectId                string `json:"projectId"`
	ApiBasePath              string `json:"apiBasePath"`
	AuthTokenSecretRef       string `json:"authTokenSecretRef"`
	AuthTokenSecretKey       string `json:"authTokenSecretKey"`
	AuthTokenSecretNamespace string `json:"authTokenSecretNamespace"`
}

func (d defaultConfigProvider) LoadConfig(cfgJSON *extapi.JSON) (StackitDnsProviderConfig, error) {
	cfg := StackitDnsProviderConfig{}

	if cfgJSON == nil {
		return cfg, fmt.Errorf("no configProvider provided")
	}

	if err := unmarshalConfig(cfgJSON, &cfg); err != nil {
		return cfg, err
	}

	if err := validateConfig(&cfg); err != nil {
		return cfg, err
	}

	setDefaultValues(&cfg)

	namespace, err := determineNamespace(cfg.AuthTokenSecretNamespace)
	if err != nil {
		return cfg, err
	}
	cfg.AuthTokenSecretNamespace = namespace

	return cfg, nil
}

func unmarshalConfig(cfgJSON *extapi.JSON, cfg *StackitDnsProviderConfig) error {
	if err := json.Unmarshal(cfgJSON.Raw, cfg); err != nil {
		return fmt.Errorf("error decoding solver configProvider: %w", err)
	}

	return nil
}

func validateConfig(cfg *StackitDnsProviderConfig) error {
	if cfg.ProjectId == "" {
		return fmt.Errorf("projectId must be specified")
	}

	return nil
}

func setDefaultValues(cfg *StackitDnsProviderConfig) {
	if cfg.ApiBasePath == "" {
		cfg.ApiBasePath = "https://dns.api.stackit.cloud"
	}
	if cfg.AuthTokenSecretRef == "" {
		cfg.AuthTokenSecretRef = "stackit-cert-manager-webhook"
	}
	if cfg.AuthTokenSecretKey == "" {
		cfg.AuthTokenSecretKey = "auth-token"
	}
}

func determineNamespace(currentNamespace string) (string, error) {
	if currentNamespace != "" {
		return currentNamespace, nil
	}

	data, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		return "", fmt.Errorf("failed to find the webhook pod namespace: %w", err)
	}

	namespace := strings.TrimSpace(string(data))
	if len(namespace) == 0 {
		return "", fmt.Errorf("invalid webhook pod namespace provided")
	}

	return namespace, nil
}
