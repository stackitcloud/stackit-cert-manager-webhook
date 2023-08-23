package resolver_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/cert-manager/cert-manager/pkg/acme/webhook/apis/acme/v1alpha1"
	repository_mock "github.com/stackitcloud/stackit-cert-manager-webhook/internal/repository/mock"
	"github.com/stackitcloud/stackit-cert-manager-webhook/internal/resolver"
	resolver_mock "github.com/stackitcloud/stackit-cert-manager-webhook/internal/resolver/mock"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/client-go/rest"
)

func TestName(t *testing.T) {
	t.Parallel()

	r := resolver.NewResolver(nil, nil, nil, nil, nil)

	assert.Equal(t, r.Name(), "stackit")
}

func TestInitialize(t *testing.T) {
	t.Parallel()

	r := resolver.NewResolver(nil, nil, nil, nil, nil)

	t.Run("successful init", func(t *testing.T) {
		t.Parallel()

		kubeConfig := &rest.Config{}
		err := r.Initialize(kubeConfig, nil)
		assert.NoError(t, err)
	})

	t.Run("unsuccessful init", func(t *testing.T) {
		t.Parallel()

		kubeConfig := &rest.Config{Burst: -1, RateLimiter: nil, QPS: 1}
		err := r.Initialize(kubeConfig, nil)
		assert.Error(t, err)
	})
}

func TestStackitDnsProviderResolver_Present(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSecretFetcher := resolver_mock.NewMockSecretFetcher(ctrl)
	mockConfigProvider := resolver_mock.NewMockConfigProvider(ctrl)
	mockZoneRepositoryFactory := repository_mock.NewMockZoneRepositoryFactory(ctrl)
	mockRRSetRepositoryFactory := repository_mock.NewMockRRSetRepositoryFactory(ctrl)

	configJson := &v1.JSON{Raw: []byte(`{"projectId":"test"}`)}
	mockConfigProvider.EXPECT().
		LoadConfig(configJson).
		Return(resolver.StackitDnsProviderConfig{}, fmt.Errorf("error decoding solver configProvider"))

	r := resolver.NewResolver(
		&http.Client{},
		mockZoneRepositoryFactory,
		mockRRSetRepositoryFactory,
		mockSecretFetcher,
		mockConfigProvider,
	)

	ch := &v1alpha1.ChallengeRequest{
		Config: configJson,
	}

	err := r.Present(ch)
	assert.Error(t, err)
}
