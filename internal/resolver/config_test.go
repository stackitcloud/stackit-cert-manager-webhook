package resolver

import (
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	d := defaultConfigProvider{}

	t.Run("nil cfgJSON", func(t *testing.T) {
		t.Parallel()

		cfg, err := d.LoadConfig(nil)
		require.Error(t, err)
		require.Equal(t, "no configProvider provided", err.Error())
		require.Equal(t, StackitDnsProviderConfig{}, cfg)
	})

	t.Run("valid cfgJSON", func(t *testing.T) {
		t.Parallel()

		rawCfg := &v1.JSON{Raw: []byte(`{"projectId":"test", "authTokenSecretNamespace": "test"}`)}

		cfg, err := d.LoadConfig(rawCfg)
		require.NoError(t, err)
		require.Equal(t, "test", cfg.ProjectId)
	})

	t.Run("not parsable cfgJSON", func(t *testing.T) {
		t.Parallel()

		rawCfg := &v1.JSON{Raw: []byte(`{"projectId":}`)}
		cfg, err := d.LoadConfig(rawCfg)
		require.Error(t, err)
		require.Contains(t, err.Error(), "error decoding solver configProvider")
		require.Equal(t, StackitDnsProviderConfig{}, cfg)
	})

	t.Run("invalid cfgJSON", func(t *testing.T) {
		t.Parallel()

		rawCfg := &v1.JSON{Raw: []byte(`{"projectId": ""}`)}
		cfg, err := d.LoadConfig(rawCfg)
		require.Error(t, err)
		require.Contains(t, err.Error(), "projectId must be specified")
		require.Equal(t, StackitDnsProviderConfig{}, cfg)
	})

	t.Run("missing projectId", func(t *testing.T) {
		t.Parallel()

		rawCfg := &v1.JSON{Raw: []byte(`{}`)}
		cfg, err := d.LoadConfig(rawCfg)
		require.Error(t, err)
		require.Equal(t, "projectId must be specified", err.Error())
		require.Equal(t, StackitDnsProviderConfig{}, cfg)
	})

	t.Run("default values set", func(t *testing.T) {
		t.Parallel()

		rawCfg := &v1.JSON{Raw: []byte(`{"projectId":"test", "authTokenSecretNamespace": "test"}`)} // Only projectId provided
		cfg, err := d.LoadConfig(rawCfg)
		require.NoError(t, err)
		require.Equal(t, "test", cfg.ProjectId)
		require.Equal(t, "https://dns.api.stackit.cloud", cfg.ApiBasePath)
		require.Equal(t, "stackit-cert-manager-webhook", cfg.AuthTokenSecretRef)
		require.Equal(t, "auth-token", cfg.AuthTokenSecretKey)
		require.Equal(t, int64(600), cfg.AcmeTxtRecordTTL)
	})

	t.Run("custom service account base url", func(t *testing.T) {
		t.Parallel()

		rawCfg := &v1.JSON{Raw: []byte(`{"projectId":"test", "authTokenSecretNamespace": "test", "serviceAccountBaseUrl": "https://custom.stackit.cloud/dns"}`)}
		cfg, err := d.LoadConfig(rawCfg)
		require.NoError(t, err)
		require.Equal(t, "test", cfg.ProjectId)
		require.Equal(t, "https://custom.stackit.cloud/dns", cfg.ServiceAccountBaseUrl)
	})
}

func TestDefaultConfigProvider_LoadConfigNamespaceFile(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	d := defaultConfigProvider{}

	t.Run("determine namespace from file", func(t *testing.T) {
		t.Parallel()

		rawCfg := &v1.JSON{Raw: []byte(`{"projectId":"test"}`)}

		f, err := os.CreateTemp("", "example")
		require.NoError(t, err)
		defer os.Remove(f.Name())
		_, err = f.Write([]byte("test-namespace"))
		require.NoError(t, err)
		err = f.Close()
		require.NoError(t, err)

		dcp := defaultConfigProvider{fileNamespaceName: f.Name()}
		cfg, err := dcp.LoadConfig(rawCfg)
		require.NoError(t, err)
		require.Equal(t, "test-namespace", cfg.AuthTokenSecretNamespace)
	})

	t.Run("fail determine namespace from file, no content", func(t *testing.T) {
		t.Parallel()

		rawCfg := &v1.JSON{Raw: []byte(`{"projectId":"test"}`)}

		f, err := os.CreateTemp("", "example")
		require.NoError(t, err)
		defer os.Remove(f.Name())
		_, err = f.Write([]byte(""))
		require.NoError(t, err)
		err = f.Close()
		require.NoError(t, err)

		dcp := defaultConfigProvider{fileNamespaceName: f.Name()}
		_, err = dcp.LoadConfig(rawCfg)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid webhook pod namespace provided")
	})

	t.Run("fail to determine namespace from file", func(t *testing.T) {
		t.Parallel()

		rawCfg := &v1.JSON{Raw: []byte(`{"projectId":"test"}`)}

		_, err := d.LoadConfig(rawCfg)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to find the webhook pod namespace")
	})
}

func TestGetRepositoryConfig_WithSaKeyPath(t *testing.T) {
	saKeyPath := "/path/to/sa/key"

	t.Setenv("STACKIT_SERVICE_ACCOUNT_KEY_PATH", saKeyPath)
	defer func() {
		t.Setenv("STACKIT_SERVICE_ACCOUNT_KEY_PATH", "")
	}()

	r := &stackitDnsProviderResolver{
		httpClient: &http.Client{},
		logger:     zap.NewNop(),
	}

	cfg := &StackitDnsProviderConfig{
		ApiBasePath:           "https://api.stackit.cloud",
		ProjectId:             "test-project",
		ServiceAccountBaseUrl: "https://sa-custom.stackit.cloud",
	}

	config, err := r.getRepositoryConfig(cfg)
	require.NoError(t, err)
	require.Equal(t, saKeyPath, config.SaKeyPath)
	require.Equal(t, "https://sa-custom.stackit.cloud", config.ServiceAccountBaseUrl)
	require.True(t, config.UseSaKey)
}

func TestGetRepositoryConfig_NoEnvSet(t *testing.T) {
	oldAuthToken := stackitAuthToken
	stackitAuthToken = "token" // global variable from resolver.go

	t.Setenv("STACKIT_SERVICE_ACCOUNT_KEY_PATH", "")
	defer func() {
		stackitAuthToken = oldAuthToken
	}()

	s := NewSecretFetcher()
	r := &stackitDnsProviderResolver{
		httpClient:    &http.Client{},
		secretFetcher: s,
		logger:        zap.NewNop(),
	}

	cfg := &StackitDnsProviderConfig{
		ApiBasePath: "https://api.stackit.cloud",
		ProjectId:   "test-project",
	}

	config, err := r.getRepositoryConfig(cfg)

	require.NoError(t, err)
	require.False(t, config.UseSaKey)
	require.Equal(t, stackitAuthToken, config.AuthToken)
}
