package resolver_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/cert-manager/cert-manager/pkg/acme/webhook"
	"github.com/cert-manager/cert-manager/pkg/acme/webhook/apis/acme/v1alpha1"
	"github.com/stackitcloud/stackit-cert-manager-webhook/internal/repository"
	repository_mock "github.com/stackitcloud/stackit-cert-manager-webhook/internal/repository/mock"
	"github.com/stackitcloud/stackit-cert-manager-webhook/internal/resolver"
	resolver_mock "github.com/stackitcloud/stackit-cert-manager-webhook/internal/resolver/mock"
	stackitdnsclient_new "github.com/stackitcloud/stackit-sdk-go/services/dns/v1api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/client-go/rest"
)

var (
	configJson       = &v1.JSON{Raw: []byte(`{"projectId":"test"}`)}
	challengeRequest = &v1alpha1.ChallengeRequest{
		Config: configJson,
	}
)

const (
	testID    = "test"
	targetKey = "delete-me"
	keepKey   = "keep-me"
)

func generateDummyPrivateKey() string {
	privKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	privKeyPEM := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privKey),
	}

	return string(pem.EncodeToMemory(privKeyPEM))
}

func TestName(t *testing.T) {
	t.Parallel()
	r := resolver.NewResolver(nil, zap.NewNop(), nil, nil, nil, nil)
	assert.Equal(t, r.Name(), "stackit")
}

func TestInitialize(t *testing.T) {
	t.Parallel()
	r := resolver.NewResolver(nil, zap.NewNop(), nil, nil, nil, nil)

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

type baseResolverSuite struct {
	suite.Suite
	ctrl                       *gomock.Controller
	mockSecretFetcher          *resolver_mock.MockSecretFetcher
	mockConfigProvider         *resolver_mock.MockConfigProvider
	mockZoneRepositoryFactory  *repository_mock.MockZoneRepositoryFactory
	mockRRSetRepositoryFactory *repository_mock.MockRRSetRepositoryFactory
	mockZoneRepository         *repository_mock.MockZoneRepository
	mockRRSetRepository        *repository_mock.MockRRSetRepository
	resolver                   webhook.Solver
	dummySAKeyJSON             string
}

func (s *baseResolverSuite) SetupTest() {
	s.mockSecretFetcher = resolver_mock.NewMockSecretFetcher(s.ctrl)
	s.mockConfigProvider = resolver_mock.NewMockConfigProvider(s.ctrl)
	s.mockZoneRepositoryFactory = repository_mock.NewMockZoneRepositoryFactory(s.ctrl)
	s.mockRRSetRepositoryFactory = repository_mock.NewMockRRSetRepositoryFactory(s.ctrl)
	s.mockZoneRepository = repository_mock.NewMockZoneRepository(s.ctrl)
	s.mockRRSetRepository = repository_mock.NewMockRRSetRepository(s.ctrl)

	dummyKey := generateDummyPrivateKey()
	s.dummySAKeyJSON = fmt.Sprintf(`{"id":"00000000-0000-0000-0000-000000000000","credentials":{"privateKey":%q}}`, dummyKey)

	s.T().Setenv("STACKIT_SERVICE_ACCOUNT_TOKEN", "dummy-token")
	s.T().Setenv("STACKIT_SERVICE_ACCOUNT_EMAIL", "test@example.com")

	wifFile, err := os.CreateTemp("", "wif-*")
	s.Require().NoError(err)
	_, _ = wifFile.Write([]byte("eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.e30.sig"))
	wifFile.Close()
	s.T().Setenv("STACKIT_FEDERATED_TOKEN_FILE", wifFile.Name())
	s.T().Cleanup(func() { os.Remove(wifFile.Name()) })

	s.resolver = resolver.NewResolver(
		&http.Client{},
		zap.NewNop(),
		s.mockZoneRepositoryFactory,
		s.mockRRSetRepositoryFactory,
		s.mockSecretFetcher,
		s.mockConfigProvider,
	)
}

func (s *baseResolverSuite) TearDownSuite() {
	s.ctrl.Finish()
}

type presentSuite struct {
	baseResolverSuite
}

//nolint:paralleltest // manipulates global environment variables
func TestPresentTestSuite(t *testing.T) {
	pSuite := new(presentSuite)
	pSuite.ctrl = gomock.NewController(t)
	suite.Run(t, pSuite)
}

func (s *presentSuite) TestConfigProviderError() {
	s.mockConfigProvider.EXPECT().
		LoadConfig(configJson).
		Return(resolver.StackitDnsProviderConfig{}, fmt.Errorf("error decoding solver configProvider"))

	err := s.resolver.Present(challengeRequest)
	s.Error(err)
}

func (s *presentSuite) TestFailGetAuthToken() {
	s.mockConfigProvider.EXPECT().
		LoadConfig(gomock.Any()).
		Return(resolver.StackitDnsProviderConfig{
			ServiceAccountSecretRef: "secret",
		}, nil)

	s.mockSecretFetcher.EXPECT().
		StringFromSecret(gomock.Any(), gomock.Any(), gomock.Any()).
		Return("", fmt.Errorf("error fetching token"))

	err := s.resolver.Present(challengeRequest)
	s.Error(err)
	s.Containsf(
		err.Error(),
		"error fetching token",
		"error message should contain error from secretFetcher",
	)
}

func (s *presentSuite) setupCommonPresentMocks() {
	s.mockZoneRepositoryFactory.EXPECT().
		NewZoneRepository(gomock.Any()).
		Return(s.mockZoneRepository, nil)

	s.mockZoneRepository.EXPECT().
		FetchZone(gomock.Any(), gomock.Any()).
		Return(&stackitdnsclient_new.Zone{Id: testID}, nil)

	s.mockRRSetRepositoryFactory.EXPECT().
		NewRRSetRepository(gomock.Any(), gomock.Any()).
		Return(s.mockRRSetRepository, nil)

	s.mockRRSetRepository.EXPECT().
		FetchRRSetForZone(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, repository.ErrRRSetNotFound)

	s.mockRRSetRepository.EXPECT().
		CreateRRSet(gomock.Any(), gomock.Any()).
		Return(nil)
}

func (s *presentSuite) TestAuthMethodSelectionDynamicSA() {
	s.mockConfigProvider.EXPECT().
		LoadConfig(gomock.Any()).
		Return(resolver.StackitDnsProviderConfig{
			ServiceAccountSecretRef:       "secret",
			ServiceAccountSecretKey:       "sa.json",
			ServiceAccountSecretNamespace: "default",
		}, nil)

	s.mockSecretFetcher.EXPECT().
		StringFromSecret("default", "secret", "sa.json").
		Return(s.dummySAKeyJSON, nil)

	s.setupCommonPresentMocks()

	err := s.resolver.Present(challengeRequest)
	s.NoError(err)
}

func (s *presentSuite) TestAuthMethodSelectionStaticSA() {
	f, err := os.CreateTemp("", "sa.json")
	s.Require().NoError(err)
	defer os.Remove(f.Name())
	_, _ = f.Write([]byte(s.dummySAKeyJSON))
	f.Close()

	s.mockConfigProvider.EXPECT().
		LoadConfig(gomock.Any()).
		Return(resolver.StackitDnsProviderConfig{
			ServiceAccountKeyPath: f.Name(),
		}, nil)

	s.setupCommonPresentMocks()

	err = s.resolver.Present(challengeRequest)
	s.NoError(err)
}

func (s *presentSuite) TestAuthMethodSelectionWIF() {
	s.mockConfigProvider.EXPECT().
		LoadConfig(gomock.Any()).
		Return(resolver.StackitDnsProviderConfig{
			UseWorkloadIdentityFederation: true,
		}, nil)

	s.setupCommonPresentMocks()

	err := s.resolver.Present(challengeRequest)
	s.NoError(err)
}

type cleanSuite struct {
	baseResolverSuite
}

//nolint:paralleltest // manipulates global environment variables
func TestCleanTestSuite(t *testing.T) {
	cSuite := new(cleanSuite)
	cSuite.ctrl = gomock.NewController(t)
	suite.Run(t, cSuite)
}

func (s *cleanSuite) setupCommonMocks() {
	s.mockConfigProvider.EXPECT().
		LoadConfig(gomock.Any()).
		Return(resolver.StackitDnsProviderConfig{}, nil)

	s.mockZoneRepositoryFactory.EXPECT().
		NewZoneRepository(gomock.Any()).
		Return(s.mockZoneRepository, nil)

	s.mockZoneRepository.EXPECT().
		FetchZone(gomock.Any(), gomock.Any()).
		Return(&stackitdnsclient_new.Zone{Id: testID}, nil)

	s.mockRRSetRepositoryFactory.EXPECT().
		NewRRSetRepository(gomock.Any(), gomock.Any()).
		Return(s.mockRRSetRepository, nil)
}

func (s *cleanSuite) TestFailFetchRRSet() {
	s.setupCommonMocks()

	s.mockRRSetRepository.EXPECT().
		FetchRRSetForZone(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, fmt.Errorf("error fetching rr set"))

	err := s.resolver.CleanUp(challengeRequest)
	s.Error(err)
	s.Containsf(
		err.Error(),
		"error fetching rr set",
		"error message should contain error from rrSetRepository",
	)
}

func (s *cleanSuite) TestFailFetchNoRRSet() {
	s.setupCommonMocks()

	s.mockRRSetRepository.EXPECT().
		FetchRRSetForZone(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, repository.ErrRRSetNotFound)

	err := s.resolver.CleanUp(challengeRequest)
	s.NoError(err)
}

func (s *cleanSuite) TestCleanUp_RemovesOnlyKey_DeletesRRSet() {
	s.setupCommonMocks()

	req := &v1alpha1.ChallengeRequest{
		Config: configJson,
		Key:    targetKey,
	}

	rrset := stackitdnsclient_new.RecordSet{
		Id: "1234",
		Records: []stackitdnsclient_new.Record{
			{Content: targetKey},
		},
	}

	s.mockRRSetRepository.EXPECT().
		FetchRRSetForZone(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&rrset, nil)

	s.mockRRSetRepository.EXPECT().
		DeleteRRSet(gomock.Any(), rrset.Id).
		Return(nil)

	err := s.resolver.CleanUp(req)
	s.NoError(err)
}

func (s *cleanSuite) TestCleanUp_RemovesOneKey_UpdatesRRSet() {
	s.setupCommonMocks()

	req := &v1alpha1.ChallengeRequest{
		Config: configJson,
		Key:    targetKey,
	}

	rrset := stackitdnsclient_new.RecordSet{
		Id: "1234",
		Records: []stackitdnsclient_new.Record{
			{Content: targetKey},
			{Content: keepKey},
		},
	}

	s.mockRRSetRepository.EXPECT().
		FetchRRSetForZone(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&rrset, nil)

	s.mockRRSetRepository.EXPECT().
		UpdateRRSet(gomock.Any(), matchedBy(func(updated stackitdnsclient_new.RecordSet) bool {
			return len(updated.Records) == 1 && updated.Records[0].Content == keepKey
		})).
		Return(nil)

	err := s.resolver.CleanUp(req)
	s.NoError(err)
}

func (s *cleanSuite) TestCleanUp_KeyNotFound_DoesNothing() {
	s.setupCommonMocks()

	req := &v1alpha1.ChallengeRequest{
		Config: configJson,
		Key:    targetKey,
	}

	rrset := stackitdnsclient_new.RecordSet{
		Id: "1234",
		Records: []stackitdnsclient_new.Record{
			{Content: keepKey},
		},
	}

	s.mockRRSetRepository.EXPECT().
		FetchRRSetForZone(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&rrset, nil)

	err := s.resolver.CleanUp(req)
	s.NoError(err)
}

func matchedBy[T any](fn func(T) bool) gomock.Matcher {
	return matcher[T]{fn}
}

type matcher[T any] struct {
	fn func(T) bool
}

func (m matcher[T]) Matches(x interface{}) bool {
	v, ok := x.(T)
	if !ok {
		return false
	}

	return m.fn(v)
}

func (m matcher[T]) String() string {
	return "custom matcher"
}
