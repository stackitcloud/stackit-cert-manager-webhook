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
	secretAccessScopeNone    = "none"
	defaultNamespaceFilePath = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
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
	AcmeTxtRecordTTL              int32  `json:"acmeTxtRecordTTL"`
}

type removedSolverFields struct {
	ServiceAccountBaseUrl    string `json:"serviceAccountBaseUrl"`
	AuthTokenSecretRef       string `json:"authTokenSecretRef"`
	AuthTokenSecretKey       string `json:"authTokenSecretKey"`
	AuthTokenSecretNamespace string `json:"authTokenSecretNamespace"`
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

	if err := d.resolveNamespace(&cfg); err != nil {
		return cfg, err
	}

	if err := validateConfig(&cfg); err != nil {
		return cfg, err
	}

	return cfg, nil
}

func (d defaultConfigProvider) resolveNamespace(cfg *StackitDnsProviderConfig) error {
	webhookNamespace, err := determineNamespace(d.fileNamespaceName)
	if err != nil {
		return err
	}

	scope := d.secretAccessScope
	if (scope == "" || scope == secretAccessScopeWebhook) && cfg.ServiceAccountSecretNamespace == "" {
		cfg.ServiceAccountSecretNamespace = webhookNamespace
	}

	if cfg.ServiceAccountSecretRef != "" {
		if err := validateSecretNamespace(cfg.ServiceAccountSecretNamespace, webhookNamespace, scope); err != nil {
			return err
		}
	}

	return nil
}

func validateSecretNamespace(configuredNamespace, webhookNamespace, accessScope string) error {
	switch accessScope {
	case "", secretAccessScopeWebhook:
		if configuredNamespace != webhookNamespace {
			return fmt.Errorf("serviceAccountSecretNamespace must be %s, got: %s", webhookNamespace, configuredNamespace)
		}

		return nil
	case secretAccessScopeIssuer:
		return nil
	case secretAccessScopeNone:
		return fmt.Errorf("dynamic secret lookup is disabled when secretAccessScope=none")
	default:
		return fmt.Errorf("invalid secretAccessScope %q", accessScope)
	}
}

func validateResourceNamespace(cfg *StackitDnsProviderConfig, resourceNamespace, accessScope string) error {
	if cfg.ServiceAccountSecretRef == "" {
		return nil
	}

	switch accessScope {
	case "", secretAccessScopeWebhook, secretAccessScopeIssuer:
		// Valid scopes for dynamic secret lookup; namespace match is enforced below.
	case secretAccessScopeNone:
		return fmt.Errorf("dynamic secret lookup is disabled when secretAccessScope=none")
	default:
		return fmt.Errorf("invalid secretAccessScope %q", accessScope)
	}

	if resourceNamespace == "" {
		return fmt.Errorf("resourceNamespace must not be empty when using serviceAccountSecretRef")
	}

	if cfg.ServiceAccountSecretNamespace == "" {
		cfg.ServiceAccountSecretNamespace = resourceNamespace
	}

	if cfg.ServiceAccountSecretNamespace != resourceNamespace {
		return fmt.Errorf("serviceAccountSecretNamespace must match issuer namespace %s, got: %s", resourceNamespace, cfg.ServiceAccountSecretNamespace)
	}

	return nil
}

func unmarshalConfig(cfgJSON *extapi.JSON, cfg *StackitDnsProviderConfig) error {
	if err := json.Unmarshal(cfgJSON.Raw, cfg); err != nil {
		return fmt.Errorf("error decoding solver configProvider: %w", err)
	}

	var removed removedSolverFields
	if err := json.Unmarshal(cfgJSON.Raw, &removed); err != nil {
		return fmt.Errorf("error decoding solver configProvider: %w", err)
	}

	return validateRemovedFields(&removed)
}

func validateRemovedFields(removed *removedSolverFields) error {
	if removed.ServiceAccountBaseUrl != "" {
		return fmt.Errorf("serviceAccountBaseUrl is no longer supported in solver config; configure STACKIT_SERVICE_ACCOUNT_BASE_URL on the webhook pod instead")
	}

	if removed.AuthTokenSecretRef != "" || removed.AuthTokenSecretKey != "" || removed.AuthTokenSecretNamespace != "" {
		return fmt.Errorf("bearer token authentication (authTokenSecretRef) was removed in v1.0.0; migrate to serviceAccountSecretRef, serviceAccountKeyPath, useWorkloadIdentityFederation, or DefaultAuth")
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

var apiBasePathRegex = regexp.MustCompile(`^https://dns\.api(?:\.[a-z0-9-]+)?\.stackit\.cloud/?$`)

func validateApiBasePath(apiBasePath string) error {
	if apiBasePathRegex.MatchString(apiBasePath) {
		return nil
	}

	return fmt.Errorf("apiBasePath not allowed: %s", apiBasePath)
}

func validateSaKeyPath(keyPath string) error {
	allowedPrefix := "/var/run/secrets/stackit/"

	clean := filepath.Clean(keyPath)

	if !strings.HasPrefix(clean, allowedPrefix) {
		return fmt.Errorf("serviceAccountKeyPath must be within %s, got: %s", allowedPrefix, clean)
	}

	return nil
}

func setDefaultValues(cfg *StackitDnsProviderConfig) {
	if cfg.ApiBasePath == "" {
		cfg.ApiBasePath = "https://dns.api.stackit.cloud"
	}

	if cfg.ServiceAccountSecretRef != "" && cfg.ServiceAccountSecretKey == "" {
		cfg.ServiceAccountSecretKey = "sa.json"
	}

	if cfg.AcmeTxtRecordTTL == 0 {
		cfg.AcmeTxtRecordTTL = 60
	}
}

func determineNamespace(fileNamespaceName string) (string, error) {
	if fileNamespaceName == defaultNamespaceFilePath {
		if envNS := strings.TrimSpace(os.Getenv("POD_NAMESPACE")); envNS != "" {
			return envNS, nil
		}
	}

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

func getSecretAccessScope() string {
	return strings.ToLower(strings.TrimSpace(os.Getenv("STACKIT_SECRET_ACCESS_SCOPE")))
}

func NewConfigProvider() ConfigProvider {
	return defaultConfigProvider{
		fileNamespaceName: defaultNamespaceFilePath,
		secretAccessScope: getSecretAccessScope(),
	}
}
