package resolver

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

const (
	testProjectID   = "test"
	testNamespace   = "test-namespace"
	testSecretName  = "secret"
	testWebhookNS   = "cert-manager"
	testTenantANS   = "tenant-a"
	testMySecretRef = "my-secret"
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
		fileName := createNamespaceFile(t, testNamespace)
		d := defaultConfigProvider{fileNamespaceName: fileName}

		cfg, err := d.LoadConfig(nil)
		require.Error(t, err)
		require.Equal(t, "no solver config provided", err.Error())
		require.Equal(t, StackitDnsProviderConfig{}, cfg)
	})

	t.Run("valid cfgJSON", func(t *testing.T) {
		t.Parallel()
		fileName := createNamespaceFile(t, testNamespace)
		d := defaultConfigProvider{fileNamespaceName: fileName}

		rawCfg := &v1.JSON{Raw: []byte(`{"projectId":"test", "serviceAccountSecretNamespace": "test-namespace"}`)}
		cfg, err := d.LoadConfig(rawCfg)
		require.NoError(t, err)
		require.Equal(t, testProjectID, cfg.ProjectId)
	})

	t.Run("not parsable cfgJSON", func(t *testing.T) {
		t.Parallel()
		fileName := createNamespaceFile(t, testNamespace)
		d := defaultConfigProvider{fileNamespaceName: fileName}

		rawCfg := &v1.JSON{Raw: []byte(`{"projectId":}`)}
		cfg, err := d.LoadConfig(rawCfg)
		require.Error(t, err)
		require.Contains(t, err.Error(), "error decoding solver config")
		require.Equal(t, StackitDnsProviderConfig{}, cfg)
	})

	t.Run("invalid cfgJSON", func(t *testing.T) {
		t.Parallel()
		fileName := createNamespaceFile(t, testNamespace)
		d := defaultConfigProvider{fileNamespaceName: fileName}

		rawCfg := &v1.JSON{Raw: []byte(`{"projectId": ""}`)}
		_, err := d.LoadConfig(rawCfg)
		require.Error(t, err)
		require.Contains(t, err.Error(), "projectId must be specified")
	})

	t.Run("default values set", func(t *testing.T) {
		t.Parallel()
		fileName := createNamespaceFile(t, testNamespace)
		d := defaultConfigProvider{fileNamespaceName: fileName}

		rawCfg := &v1.JSON{Raw: []byte(`{"projectId":"test", "serviceAccountSecretRef":"my-secret", "serviceAccountSecretNamespace": "test-namespace"}`)}
		cfg, err := d.LoadConfig(rawCfg)
		require.NoError(t, err)
		require.Equal(t, testProjectID, cfg.ProjectId)
		require.Equal(t, "https://dns.api.stackit.cloud", cfg.ApiBasePath)
		require.Equal(t, "sa.json", cfg.ServiceAccountSecretKey)
		require.Equal(t, int32(60), cfg.AcmeTxtRecordTTL)
	})
}

func TestDetermineAuthType(t *testing.T) {
	t.Parallel()

	t.Run("default auth when none specified", func(t *testing.T) {
		t.Parallel()
		authType, err := determineAuthType(&StackitDnsProviderConfig{ProjectId: testProjectID})
		require.NoError(t, err)
		require.Equal(t, AuthTypeDefault, authType)
	})

	t.Run("dynamic SA auth when secret ref specified", func(t *testing.T) {
		t.Parallel()
		authType, err := determineAuthType(&StackitDnsProviderConfig{
			ProjectId:               testProjectID,
			ServiceAccountSecretRef: testMySecretRef,
		})
		require.NoError(t, err)
		require.Equal(t, AuthTypeDynamicSA, authType)
	})

	t.Run("static SA auth when key path specified", func(t *testing.T) {
		t.Parallel()
		authType, err := determineAuthType(&StackitDnsProviderConfig{
			ProjectId:             testProjectID,
			ServiceAccountKeyPath: "/var/run/secrets/stackit/sa.json",
		})
		require.NoError(t, err)
		require.Equal(t, AuthTypeStaticSA, authType)
	})

	t.Run("WIF auth when enabled", func(t *testing.T) {
		t.Parallel()
		authType, err := determineAuthType(&StackitDnsProviderConfig{
			ProjectId:                     testProjectID,
			UseWorkloadIdentityFederation: true,
		})
		require.NoError(t, err)
		require.Equal(t, AuthTypeWIF, authType)
	})

	t.Run("ambiguous auth when multiple specified", func(t *testing.T) {
		t.Parallel()
		_, err := determineAuthType(&StackitDnsProviderConfig{
			ProjectId:                     testProjectID,
			ServiceAccountSecretRef:       testMySecretRef,
			UseWorkloadIdentityFederation: true,
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "ambiguous authentication configuration")
	})
}

func TestSecretAccessScopeValidation(t *testing.T) {
	t.Parallel()

	t.Run("webhook scope rejects different namespace", func(t *testing.T) {
		t.Parallel()
		fileName := createNamespaceFile(t, testWebhookNS)
		d := defaultConfigProvider{
			fileNamespaceName: fileName,
			secretAccessScope: secretAccessScopeWebhook,
		}

		rawCfg := &v1.JSON{Raw: []byte(`{"projectId":"test", "serviceAccountSecretRef":"secret", "serviceAccountSecretNamespace":"other-ns"}`)}
		_, err := d.LoadConfig(rawCfg)
		require.Error(t, err)
		require.Contains(t, err.Error(), "serviceAccountSecretNamespace must be cert-manager, got: other-ns")
	})

	t.Run("issuer scope allows tenant namespace", func(t *testing.T) {
		t.Parallel()
		fileName := createNamespaceFile(t, testWebhookNS)
		d := defaultConfigProvider{
			fileNamespaceName: fileName,
			secretAccessScope: secretAccessScopeIssuer,
		}

		rawCfg := &v1.JSON{Raw: []byte(`{"projectId":"test", "serviceAccountSecretRef":"secret", "serviceAccountSecretNamespace":"tenant-ns"}`)}
		cfg, err := d.LoadConfig(rawCfg)
		require.NoError(t, err)
		require.Equal(t, "tenant-ns", cfg.ServiceAccountSecretNamespace)
	})

	t.Run("issuer scope allows empty namespace to be defaulted from resourceNamespace later", func(t *testing.T) {
		t.Parallel()
		fileName := createNamespaceFile(t, testWebhookNS)
		d := defaultConfigProvider{
			fileNamespaceName: fileName,
			secretAccessScope: secretAccessScopeIssuer,
		}

		rawCfg := &v1.JSON{Raw: []byte(`{"projectId":"test", "serviceAccountSecretRef":"secret"}`)}
		cfg, err := d.LoadConfig(rawCfg)
		require.NoError(t, err)
		require.Empty(t, cfg.ServiceAccountSecretNamespace)
	})

	t.Run("none scope rejects dynamic secret lookup", func(t *testing.T) {
		t.Parallel()
		fileName := createNamespaceFile(t, testWebhookNS)
		d := defaultConfigProvider{
			fileNamespaceName: fileName,
			secretAccessScope: secretAccessScopeNone,
		}

		rawCfg := &v1.JSON{Raw: []byte(`{"projectId":"test", "serviceAccountSecretRef":"secret"}`)}
		_, err := d.LoadConfig(rawCfg)
		require.Error(t, err)
		require.Contains(t, err.Error(), "dynamic secret lookup is disabled when secretAccessScope=none")
	})

	t.Run("invalid secretAccessScope rejected", func(t *testing.T) {
		t.Parallel()
		fileName := createNamespaceFile(t, testWebhookNS)
		d := defaultConfigProvider{
			fileNamespaceName: fileName,
			secretAccessScope: "invalid-scope",
		}

		rawCfg := &v1.JSON{Raw: []byte(`{"projectId":"test", "serviceAccountSecretRef":"secret", "serviceAccountSecretNamespace":"cert-manager"}`)}
		_, err := d.LoadConfig(rawCfg)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid secretAccessScope")
	})
}

func TestSecretAccessScopeWithoutSecretRef(t *testing.T) {
	t.Parallel()

	t.Run("invalid secretAccessScope rejected even without serviceAccountSecretRef", func(t *testing.T) {
		t.Parallel()
		d := defaultConfigProvider{
			secretAccessScope: "invalid-scope",
		}

		rawCfg := &v1.JSON{Raw: []byte(`{"projectId":"test"}`)}
		_, err := d.LoadConfig(rawCfg)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid secretAccessScope")
	})

	t.Run("none and issuer scopes do not require pod namespace file", func(t *testing.T) {
		t.Parallel()
		for _, scope := range []string{secretAccessScopeNone, secretAccessScopeIssuer} {
			d := defaultConfigProvider{
				fileNamespaceName: "/nonexistent/namespace/file",
				secretAccessScope: scope,
			}

			rawCfg := &v1.JSON{Raw: []byte(`{"projectId":"test"}`)}
			cfg, err := d.LoadConfig(rawCfg)
			require.NoError(t, err)
			require.Equal(t, testProjectID, cfg.ProjectId)
		}
	})
}

func TestValidateResourceNamespace(t *testing.T) {
	t.Parallel()

	t.Run("issuer scope allows matching resource namespace", func(t *testing.T) {
		t.Parallel()
		cfg := &StackitDnsProviderConfig{
			ServiceAccountSecretRef:       testSecretName,
			ServiceAccountSecretNamespace: testTenantANS,
		}
		err := validateResourceNamespace(cfg, testTenantANS, secretAccessScopeIssuer)
		require.NoError(t, err)
	})

	t.Run("issuer scope rejects cross-tenant resource namespace", func(t *testing.T) {
		t.Parallel()
		cfg := &StackitDnsProviderConfig{
			ServiceAccountSecretRef:       testSecretName,
			ServiceAccountSecretNamespace: "tenant-b",
		}
		err := validateResourceNamespace(cfg, testTenantANS, secretAccessScopeIssuer)
		require.Error(t, err)
		require.Contains(t, err.Error(), "serviceAccountSecretNamespace must match issuer namespace tenant-a, got: tenant-b")
	})

	t.Run("issuer scope defaults empty secret namespace to resource namespace", func(t *testing.T) {
		t.Parallel()
		cfg := &StackitDnsProviderConfig{
			ServiceAccountSecretRef: testSecretName,
		}
		err := validateResourceNamespace(cfg, testTenantANS, secretAccessScopeIssuer)
		require.NoError(t, err)
		require.Equal(t, testTenantANS, cfg.ServiceAccountSecretNamespace)
	})

	t.Run("none scope rejects dynamic secret lookup", func(t *testing.T) {
		t.Parallel()
		cfg := &StackitDnsProviderConfig{
			ServiceAccountSecretRef: testSecretName,
		}
		err := validateResourceNamespace(cfg, testTenantANS, secretAccessScopeNone)
		require.Error(t, err)
		require.Contains(t, err.Error(), "dynamic secret lookup is disabled when secretAccessScope=none")
	})

	t.Run("webhook scope allows ClusterIssuer in webhook namespace", func(t *testing.T) {
		t.Parallel()
		cfg := &StackitDnsProviderConfig{
			ServiceAccountSecretRef:       testSecretName,
			ServiceAccountSecretNamespace: testWebhookNS,
		}
		err := validateResourceNamespace(cfg, testWebhookNS, secretAccessScopeWebhook)
		require.NoError(t, err)
	})

	t.Run("webhook scope rejects tenant Issuer accessing webhook namespace secret", func(t *testing.T) {
		t.Parallel()
		cfg := &StackitDnsProviderConfig{
			ServiceAccountSecretRef:       testSecretName,
			ServiceAccountSecretNamespace: testWebhookNS,
		}
		err := validateResourceNamespace(cfg, testTenantANS, secretAccessScopeWebhook)
		require.Error(t, err)
		require.Contains(t, err.Error(), "serviceAccountSecretNamespace must match issuer namespace tenant-a, got: cert-manager")
	})

	t.Run("empty resource namespace rejected when secretRef is set", func(t *testing.T) {
		t.Parallel()
		cfg := &StackitDnsProviderConfig{
			ServiceAccountSecretRef:       testSecretName,
			ServiceAccountSecretNamespace: testWebhookNS,
		}
		err := validateResourceNamespace(cfg, "", secretAccessScopeWebhook)
		require.Error(t, err)
		require.Contains(t, err.Error(), "resourceNamespace must not be empty")
	})
}

func TestRemovedConfigFieldsRejected(t *testing.T) {
	t.Parallel()

	t.Run("serviceAccountBaseUrl in solver config is rejected", func(t *testing.T) {
		t.Parallel()
		fileName := createNamespaceFile(t, testNamespace)
		d := defaultConfigProvider{fileNamespaceName: fileName}

		rawCfg := &v1.JSON{Raw: []byte(`{"projectId":"test", "serviceAccountBaseUrl":"https://evil.example.com/token"}`)}
		_, err := d.LoadConfig(rawCfg)
		require.Error(t, err)
		require.Contains(t, err.Error(), "serviceAccountBaseUrl is no longer supported in solver config")
	})

	t.Run("legacy authTokenSecretRef in solver config is rejected with migration hint", func(t *testing.T) {
		t.Parallel()
		fileName := createNamespaceFile(t, testNamespace)
		d := defaultConfigProvider{fileNamespaceName: fileName}

		rawCfg := &v1.JSON{Raw: []byte(`{"projectId":"test", "authTokenSecretRef":"old-token-secret"}`)}
		_, err := d.LoadConfig(rawCfg)
		require.Error(t, err)
		require.Contains(t, err.Error(), "bearer token authentication (authTokenSecretRef) was removed in v1.0.0")
	})
}

func TestValidateApiBasePathAndSaKeyPath(t *testing.T) {
	t.Parallel()

	t.Run("valid global and regional apiBasePath", func(t *testing.T) {
		t.Parallel()
		require.NoError(t, validateApiBasePath("https://dns.api.stackit.cloud"))
		require.NoError(t, validateApiBasePath("https://dns.api.eu01.stackit.cloud/"))
	})

	t.Run("disallowed apiBasePath rejected", func(t *testing.T) {
		t.Parallel()
		err := validateApiBasePath("https://evil.example.com")
		require.Error(t, err)
		require.Contains(t, err.Error(), "apiBasePath not allowed")
	})

	t.Run("valid serviceAccountKeyPath allowed", func(t *testing.T) {
		t.Parallel()
		require.NoError(t, validateSaKeyPath("/var/run/secrets/stackit/sa.json"))
	})

	t.Run("invalid or traversal serviceAccountKeyPath rejected", func(t *testing.T) {
		t.Parallel()
		err := validateSaKeyPath("/var/run/secrets/stackit/../../etc/passwd")
		require.Error(t, err)
		require.Contains(t, err.Error(), "serviceAccountKeyPath must be within /var/run/secrets/stackit/")

		err = validateSaKeyPath("/var/run/secrets/stackit")
		require.Error(t, err)
		require.Contains(t, err.Error(), "serviceAccountKeyPath must be within /var/run/secrets/stackit/")

		err = validateSaKeyPath("/etc/shadow")
		require.Error(t, err)
		require.Contains(t, err.Error(), "serviceAccountKeyPath must be within /var/run/secrets/stackit/")
	})
}

func TestDefaultConfigProvider_LoadConfigNamespaceFile(t *testing.T) {
	t.Parallel()

	t.Run("determine namespace from file", func(t *testing.T) {
		t.Parallel()
		fileName := createNamespaceFile(t, testNamespace)
		dcp := defaultConfigProvider{fileNamespaceName: fileName}

		rawCfg := &v1.JSON{Raw: []byte(`{"projectId":"test"}`)}
		cfg, err := dcp.LoadConfig(rawCfg)
		require.NoError(t, err)
		require.Equal(t, testNamespace, cfg.ServiceAccountSecretNamespace)
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
