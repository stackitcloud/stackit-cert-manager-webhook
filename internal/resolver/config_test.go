package resolver

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	d := defaultConfigProvider{}

	t.Run("nil cfgJSON", func(t *testing.T) {
		t.Parallel()

		cfg, err := d.LoadConfig(nil)
		assert.Error(t, err)
		assert.Equal(t, "no configProvider provided", err.Error())
		assert.Equal(t, StackitDnsProviderConfig{}, cfg)
	})

	t.Run("valid cfgJSON", func(t *testing.T) {
		t.Parallel()

		rawCfg := &v1.JSON{Raw: []byte(`{"projectId":"test", "authTokenSecretNamespace": "test"}`)}

		cfg, err := d.LoadConfig(rawCfg)
		assert.NoError(t, err)
		assert.Equal(t, "test", cfg.ProjectId)
	})

	t.Run("not parsable cfgJSON", func(t *testing.T) {
		t.Parallel()

		rawCfg := &v1.JSON{Raw: []byte(`{"projectId":}`)}
		cfg, err := d.LoadConfig(rawCfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "error decoding solver configProvider")
		assert.Equal(t, StackitDnsProviderConfig{}, cfg)
	})

	t.Run("invalid cfgJSON", func(t *testing.T) {
		t.Parallel()

		rawCfg := &v1.JSON{Raw: []byte(`{"projectId": ""}`)}
		cfg, err := d.LoadConfig(rawCfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "projectId must be specified")
		assert.Equal(t, StackitDnsProviderConfig{}, cfg)
	})

	t.Run("missing projectId", func(t *testing.T) {
		t.Parallel()

		rawCfg := &v1.JSON{Raw: []byte(`{}`)}
		cfg, err := d.LoadConfig(rawCfg)
		assert.Error(t, err)
		assert.Equal(t, "projectId must be specified", err.Error())
		assert.Equal(t, StackitDnsProviderConfig{}, cfg)
	})

	t.Run("default values set", func(t *testing.T) {
		t.Parallel()

		rawCfg := &v1.JSON{Raw: []byte(`{"projectId":"test", "authTokenSecretNamespace": "test"}`)} // Only projectId provided
		cfg, err := d.LoadConfig(rawCfg)
		assert.NoError(t, err)
		assert.Equal(t, "test", cfg.ProjectId)
		assert.Equal(t, "https://dns.api.stackit.cloud", cfg.ApiBasePath)
		assert.Equal(t, "stackit-cert-manager-webhook", cfg.AuthTokenSecretRef)
		assert.Equal(t, "auth-token", cfg.AuthTokenSecretKey)
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
		assert.NoError(t, err)
		defer os.Remove(f.Name())
		_, err = f.Write([]byte("test-namespace"))
		assert.NoError(t, err)
		err = f.Close()
		assert.NoError(t, err)

		dcp := defaultConfigProvider{fileNamespaceName: f.Name()}
		cfg, err := dcp.LoadConfig(rawCfg)
		assert.NoError(t, err)
		assert.Equal(t, "test-namespace", cfg.AuthTokenSecretNamespace)
	})

	t.Run("fail determine namespace from file, no content", func(t *testing.T) {
		t.Parallel()

		rawCfg := &v1.JSON{Raw: []byte(`{"projectId":"test"}`)}

		f, err := os.CreateTemp("", "example")
		assert.NoError(t, err)
		defer os.Remove(f.Name())
		_, err = f.Write([]byte(""))
		assert.NoError(t, err)
		err = f.Close()
		assert.NoError(t, err)

		dcp := defaultConfigProvider{fileNamespaceName: f.Name()}
		_, err = dcp.LoadConfig(rawCfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid webhook pod namespace provided")
	})

	t.Run("fail to determine namespace from file", func(t *testing.T) {
		t.Parallel()

		rawCfg := &v1.JSON{Raw: []byte(`{"projectId":"test"}`)}

		_, err := d.LoadConfig(rawCfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to find the webhook pod namespace")
	})
}
