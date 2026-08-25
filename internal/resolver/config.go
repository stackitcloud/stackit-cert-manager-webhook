package resolver

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	extapi "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

//go:generate mockgen -destination=./mock/config.go -source=./config.go ConfigProvider
type ConfigProvider interface {
	LoadConfig(cfgJSON *extapi.JSON) (StackitDnsProviderConfig, error)
}

type defaultConfigProvider struct {
	fileNamespaceName string
	secretAccessScope string
}

const (
	secretAccessScopeWebhook = "webhook"
	secretAccessScopeIssuer  = "issuer"
)

type AuthType int

const (
	AuthTypeDefault AuthType = iota
	AuthTypeDynamicSA
	AuthTypeStaticSA
	AuthTypeWIF
)

type StackitDnsProviderConfig struct {
	ProjectId                     string `json:"projectId"`
	ApiBasePath                   string `json:"apiBasePath"`
	ServiceAccountSecretRef       string `json:"serviceAccountSecretRef"`
	ServiceAccountSecretKey       string `json:"serviceAccountSecretKey"`
	ServiceAccountSecretNamespace string `json:"serviceAccountSecretNamespace"`
	ServiceAccountKeyPath         string `json:"serviceAccountKeyPath"`
	UseWorkloadIdentityFederation bool   `json:"useWorkloadIdentityFederation"`
	ServiceAccountBaseUrl         string `json:"serviceAccountBaseUrl"`
	AcmeTxtRecordTTL              int32  `json:"acmeTxtRecordTTL"`
}

func determineAuthType(cfg *StackitDnsProviderConfig) (AuthType, error) {
	var activeTypes []AuthType

	if len(cfg.ServiceAccountSecretRef) > 0 {
		activeTypes = append(activeTypes, AuthTypeDynamicSA)
	}
	if len(cfg.ServiceAccountKeyPath) > 0 {
		activeTypes = append(activeTypes, AuthTypeStaticSA)
	}
	if cfg.UseWorkloadIdentityFederation {
		activeTypes = append(activeTypes, AuthTypeWIF)
	}

	if len(activeTypes) > 1 {
		return AuthTypeDefault, fmt.Errorf("ambiguous authentication configuration: specify at most one of serviceAccountSecretRef, serviceAccountKeyPath, or useWorkloadIdentityFederation")
	}

	if len(activeTypes) == 1 {
		return activeTypes[0], nil
	}

	return AuthTypeDefault, nil
}

func (d defaultConfigProvider) LoadConfig(cfgJSON *extapi.JSON) (StackitDnsProviderConfig, error) {
	cfg := StackitDnsProviderConfig{}

	if cfgJSON == nil {
		return cfg, fmt.Errorf("no configProvider provided")
	}

	if err := unmarshalConfig(cfgJSON, &cfg); err != nil {
		return cfg, err
	}

	setDefaultValues(&cfg)

	webhookNamespace, err := determineNamespace(d.fileNamespaceName)
	if err != nil {
		return cfg, err
	}

	scope := d.secretAccessScope
	if scope == "" || scope == secretAccessScopeWebhook {
		if cfg.ServiceAccountSecretNamespace == "" {
			cfg.ServiceAccountSecretNamespace = webhookNamespace
		}

		if err := validateSecretNamespace(cfg.ServiceAccountSecretNamespace, webhookNamespace, scope); err != nil {
			return cfg, err
		}
	}

	if err := validateConfig(&cfg); err != nil {
		return cfg, err
	}

	return cfg, nil
}

func validateSecretNamespace(configuredNamespace, webhookNamespace, accessScope string) error {
	switch accessScope {
	case "", secretAccessScopeWebhook:
		if configuredNamespace != webhookNamespace {
			return fmt.Errorf("serviceAccountSecretNamespace must be %s, got: %s", webhookNamespace, configuredNamespace)
		}

		return nil
	case secretAccessScopeIssuer:
		if configuredNamespace == "" {
			return fmt.Errorf("serviceAccountSecretNamespace must be specified when secretAccessScope=issuer")
		}

		return nil
	default:
		return fmt.Errorf("invalid secretAccessScope %q", accessScope)
	}
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

	if err := validateApiBasePath(cfg.ApiBasePath); err != nil {
		return err
	}

	if cfg.ServiceAccountKeyPath != "" {
		if err := validateSaKeyPath(cfg.ServiceAccountKeyPath); err != nil {
			return err
		}
	}

	return nil
}

func validateApiBasePath(apiBasePath string) error {
	pattern := "^https://dns\\.api(?:\\.[a-z0-9-]+)?\\.stackit\\.cloud/?$"

	if matched, err := regexp.MatchString(pattern, apiBasePath); err == nil && matched {
		return nil
	}

	return fmt.Errorf("apiBasePath not allowed: %s", apiBasePath)
}

func validateSaKeyPath(keyPath string) error {
	allowedPrefix := "/var/run/secrets/stackit/"

	clean := filepath.Clean(keyPath)

	if !strings.HasPrefix(clean, allowedPrefix) && clean != strings.TrimSuffix(allowedPrefix, "/") {
		return fmt.Errorf("serviceAccountKeyPath must be within %s, got: %s", allowedPrefix, clean)
	}

	return nil
}

func setDefaultValues(cfg *StackitDnsProviderConfig) {
	if cfg.ApiBasePath == "" {
		cfg.ApiBasePath = "https://dns.api.stackit.cloud"
	}

	if cfg.AcmeTxtRecordTTL == 0 {
		cfg.AcmeTxtRecordTTL = 600
	}
}

func determineNamespace(fileNamespaceName string) (string, error) {
	data, err := os.ReadFile(fileNamespaceName)
	if err != nil {
		return "", fmt.Errorf("failed to find the webhook pod namespace: %w", err)
	}

	namespace := strings.TrimSpace(string(data))
	if len(namespace) == 0 {
		return "", fmt.Errorf("invalid webhook pod namespace provided")
	}

	return namespace, nil
}

func NewConfigProvider() ConfigProvider {
	return defaultConfigProvider{
		fileNamespaceName: "/var/run/secrets/kubernetes.io/serviceaccount/namespace",
		secretAccessScope: strings.ToLower(strings.TrimSpace(os.Getenv("STACKIT_SECRET_ACCESS_SCOPE"))),
	}
}
