package resolver

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

func createNamespaceFile(t *testing.T, content string) string {
	t.Helper()

	f, err := os.CreateTemp("", "namespace-*")
	require.NoError(t, err)

	t.Cleanup(func() {
		os.Remove(f.Name())
	})

	_, err = f.Write([]byte(content))
	require.NoError(t, err)

	err = f.Close()
	require.NoError(t, err)

	return f.Name()
}

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	t.Run("nil cfgJSON", func(t *testing.T) {
		t.Parallel()
		fileName := createNamespaceFile(t, "test-namespace")
		d := defaultConfigProvider{fileNamespaceName: fileName}

		cfg, err := d.LoadConfig(nil)
		require.Error(t, err)
		require.Equal(t, "no configProvider provided", err.Error())
		require.Equal(t, StackitDnsProviderConfig{}, cfg)
	})

	t.Run("valid cfgJSON", func(t *testing.T) {
		t.Parallel()
		fileName := createNamespaceFile(t, "test-namespace")
		d := defaultConfigProvider{fileNamespaceName: fileName}

		rawCfg := &v1.JSON{Raw: []byte(`{"projectId":"test", "serviceAccountSecretNamespace": "test-namespace"}`)}
		cfg, err := d.LoadConfig(rawCfg)
		require.NoError(t, err)
		require.Equal(t, "test", cfg.ProjectId)
	})

	t.Run("not parsable cfgJSON", func(t *testing.T) {
		t.Parallel()
		fileName := createNamespaceFile(t, "test-namespace")
		d := defaultConfigProvider{fileNamespaceName: fileName}

		rawCfg := &v1.JSON{Raw: []byte(`{"projectId":}`)}
		cfg, err := d.LoadConfig(rawCfg)
		require.Error(t, err)
		require.Contains(t, err.Error(), "error decoding solver configProvider")
		require.Equal(t, StackitDnsProviderConfig{}, cfg)
	})

	t.Run("invalid cfgJSON", func(t *testing.T) {
		t.Parallel()
		fileName := createNamespaceFile(t, "test-namespace")
		d := defaultConfigProvider{fileNamespaceName: fileName}

		rawCfg := &v1.JSON{Raw: []byte(`{"projectId": ""}`)}
		_, err := d.LoadConfig(rawCfg)
		require.Error(t, err)
		require.Contains(t, err.Error(), "projectId must be specified")
	})

	t.Run("default values set", func(t *testing.T) {
		t.Parallel()
		fileName := createNamespaceFile(t, "test-namespace")
		d := defaultConfigProvider{fileNamespaceName: fileName}

		rawCfg := &v1.JSON{Raw: []byte(`{"projectId":"test", "serviceAccountSecretNamespace": "test-namespace"}`)}
		cfg, err := d.LoadConfig(rawCfg)
		require.NoError(t, err)
		require.Equal(t, "test", cfg.ProjectId)
		require.Equal(t, "https://dns.api.stackit.cloud", cfg.ApiBasePath)
		require.Equal(t, int32(600), cfg.AcmeTxtRecordTTL)
	})
}

func TestDefaultConfigProvider_LoadConfigNamespaceFile(t *testing.T) {
	t.Parallel()

	t.Run("determine namespace from file", func(t *testing.T) {
		t.Parallel()
		fileName := createNamespaceFile(t, "test-namespace")
		dcp := defaultConfigProvider{fileNamespaceName: fileName}

		rawCfg := &v1.JSON{Raw: []byte(`{"projectId":"test"}`)}
		cfg, err := dcp.LoadConfig(rawCfg)
		require.NoError(t, err)
		require.Equal(t, "test-namespace", cfg.ServiceAccountSecretNamespace)
	})

	t.Run("fail determine namespace from file, no content", func(t *testing.T) {
		t.Parallel()
		fileName := createNamespaceFile(t, "")
		dcp := defaultConfigProvider{fileNamespaceName: fileName}

		rawCfg := &v1.JSON{Raw: []byte(`{"projectId":"test"}`)}
		_, err := dcp.LoadConfig(rawCfg)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid webhook pod namespace provided")
	})
}
