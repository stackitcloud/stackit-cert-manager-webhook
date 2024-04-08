package resolver_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/cert-manager/cert-manager/pkg/acme/webhook"
	"github.com/cert-manager/cert-manager/pkg/acme/webhook/apis/acme/v1alpha1"
	"github.com/stackitcloud/stackit-cert-manager-webhook/internal/repository"
	repository_mock "github.com/stackitcloud/stackit-cert-manager-webhook/internal/repository/mock"
	"github.com/stackitcloud/stackit-cert-manager-webhook/internal/resolver"
	resolver_mock "github.com/stackitcloud/stackit-cert-manager-webhook/internal/resolver/mock"
	stackitdnsclient_new "github.com/stackitcloud/stackit-sdk-go/services/dns"
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

type presentSuite struct {
	suite.Suite
	ctrl                       *gomock.Controller
	mockSecretFetcher          *resolver_mock.MockSecretFetcher
	mockConfigProvider         *resolver_mock.MockConfigProvider
	mockZoneRepositoryFactory  *repository_mock.MockZoneRepositoryFactory
	mockRRSetRepositoryFactory *repository_mock.MockRRSetRepositoryFactory
	mockZoneRepository         *repository_mock.MockZoneRepository
	mockRRSetRepository        *repository_mock.MockRRSetRepository
	resolver                   webhook.Solver
}

func (s *presentSuite) SetupTest() {
	s.mockSecretFetcher = resolver_mock.NewMockSecretFetcher(s.ctrl)
	s.mockConfigProvider = resolver_mock.NewMockConfigProvider(s.ctrl)
	s.mockZoneRepositoryFactory = repository_mock.NewMockZoneRepositoryFactory(s.ctrl)
	s.mockRRSetRepositoryFactory = repository_mock.NewMockRRSetRepositoryFactory(s.ctrl)
	s.mockZoneRepository = repository_mock.NewMockZoneRepository(s.ctrl)
	s.mockRRSetRepository = repository_mock.NewMockRRSetRepository(s.ctrl)

	s.resolver = resolver.NewResolver(
		&http.Client{},
		zap.NewNop(),
		s.mockZoneRepositoryFactory,
		s.mockRRSetRepositoryFactory,
		s.mockSecretFetcher,
		s.mockConfigProvider,
	)
}

func (s *presentSuite) TearDownSuite() {
	s.ctrl.Finish()
}

func TestPresentTestSuite(t *testing.T) {
	t.Parallel()

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
		Return(resolver.StackitDnsProviderConfig{}, nil)
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

func (s *presentSuite) TestFailFetchZone() {
	s.mockConfigProvider.EXPECT().
		LoadConfig(gomock.Any()).
		Return(resolver.StackitDnsProviderConfig{}, nil)
	s.mockSecretFetcher.EXPECT().
		StringFromSecret(gomock.Any(), gomock.Any(), gomock.Any()).
		Return("", nil)
	s.mockZoneRepositoryFactory.EXPECT().
		NewZoneRepository(gomock.Any()).
		Return(s.mockZoneRepository, nil)
	s.mockZoneRepository.EXPECT().
		FetchZone(gomock.Any(), gomock.Any()).
		Return(nil, fmt.Errorf("error fetching zone"))

	err := s.resolver.Present(challengeRequest)
	s.Error(err)
	s.Containsf(
		err.Error(),
		"error fetching zone",
		"error message should contain error from zoneRepository",
	)
}

func (s *presentSuite) TestFailFetchRRSet() {
	s.mockConfigProvider.EXPECT().
		LoadConfig(gomock.Any()).
		Return(resolver.StackitDnsProviderConfig{}, nil)
	s.mockSecretFetcher.EXPECT().
		StringFromSecret(gomock.Any(), gomock.Any(), gomock.Any()).
		Return("", nil)
	s.mockZoneRepositoryFactory.EXPECT().
		NewZoneRepository(gomock.Any()).
		Return(s.mockZoneRepository, nil)
	s.mockZoneRepository.EXPECT().
		FetchZone(gomock.Any(), gomock.Any()).
		Return(&stackitdnsclient_new.Zone{Id: toPtr("test")}, nil)
	s.mockRRSetRepositoryFactory.EXPECT().
		NewRRSetRepository(gomock.Any(), gomock.Any()).
		Return(s.mockRRSetRepository, nil)
	s.mockRRSetRepository.EXPECT().
		FetchRRSetForZone(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, fmt.Errorf("error fetching rr set"))

	err := s.resolver.Present(challengeRequest)
	s.Error(err)
	s.Containsf(
		err.Error(),
		"error fetching rr set",
		"error message should contain error from rrSetRepository",
	)
}

func (s *presentSuite) TestSuccessCreateRRSet() {
	s.mockConfigProvider.EXPECT().
		LoadConfig(gomock.Any()).
		Return(resolver.StackitDnsProviderConfig{}, nil)
	s.mockSecretFetcher.EXPECT().
		StringFromSecret(gomock.Any(), gomock.Any(), gomock.Any()).
		Return("", nil)
	s.mockZoneRepositoryFactory.EXPECT().
		NewZoneRepository(gomock.Any()).
		Return(s.mockZoneRepository, nil)
	s.mockZoneRepository.EXPECT().
		FetchZone(gomock.Any(), gomock.Any()).
		Return(&stackitdnsclient_new.Zone{Id: toPtr("test")}, nil)
	s.mockRRSetRepositoryFactory.EXPECT().
		NewRRSetRepository(gomock.Any(), gomock.Any()).
		Return(s.mockRRSetRepository, nil)
	s.mockRRSetRepository.EXPECT().
		FetchRRSetForZone(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, repository.ErrRRSetNotFound)
	s.mockRRSetRepository.EXPECT().
		CreateRRSet(gomock.Any(), gomock.Any()).
		Return(nil)

	err := s.resolver.Present(challengeRequest)
	s.NoError(err)
}

func (s *presentSuite) TestSuccessUpdateRRSet() {
	s.mockConfigProvider.EXPECT().
		LoadConfig(gomock.Any()).
		Return(resolver.StackitDnsProviderConfig{}, nil)
	s.mockSecretFetcher.EXPECT().
		StringFromSecret(gomock.Any(), gomock.Any(), gomock.Any()).
		Return("", nil)
	s.mockZoneRepositoryFactory.EXPECT().
		NewZoneRepository(gomock.Any()).
		Return(s.mockZoneRepository, nil)
	s.mockZoneRepository.EXPECT().
		FetchZone(gomock.Any(), gomock.Any()).
		Return(&stackitdnsclient_new.Zone{Id: toPtr("test")}, nil)
	s.mockRRSetRepositoryFactory.EXPECT().
		NewRRSetRepository(gomock.Any(), gomock.Any()).
		Return(s.mockRRSetRepository, nil)
	s.mockRRSetRepository.EXPECT().
		FetchRRSetForZone(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&stackitdnsclient_new.RecordSet{}, nil)
	s.mockRRSetRepository.EXPECT().
		UpdateRRSet(gomock.Any(), gomock.Any()).
		Return(nil)

	err := s.resolver.Present(challengeRequest)
	s.NoError(err)
}

type cleanSuite struct {
	presentSuite
}

func TestCleanTestSuite(t *testing.T) {
	t.Parallel()

	cSuite := new(cleanSuite)
	cSuite.ctrl = gomock.NewController(t)

	suite.Run(t, cSuite)
}

func (s *cleanSuite) setupCommonMocks() {
	s.mockConfigProvider.EXPECT().
		LoadConfig(gomock.Any()).
		Return(resolver.StackitDnsProviderConfig{}, nil)
	s.mockSecretFetcher.EXPECT().
		StringFromSecret(gomock.Any(), gomock.Any(), gomock.Any()).
		Return("", nil)
	s.mockZoneRepositoryFactory.EXPECT().
		NewZoneRepository(gomock.Any()).
		Return(s.mockZoneRepository, nil)
	s.mockZoneRepository.EXPECT().
		FetchZone(gomock.Any(), gomock.Any()).
		Return(&stackitdnsclient_new.Zone{Id: toPtr("test")}, nil)
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

func (s *cleanSuite) TestFailDeleteNoRRSet() {
	s.setupCommonMocks()
	rrset := stackitdnsclient_new.RecordSet{
		Id: toPtr("1234"),
	}
	s.mockRRSetRepository.EXPECT().
		FetchRRSetForZone(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&rrset, nil)
	s.mockRRSetRepository.EXPECT().
		DeleteRRSet(gomock.Any(), *rrset.Id).
		Return(repository.ErrRRSetNotFound)

	err := s.resolver.CleanUp(challengeRequest)
	s.NoError(err)
}

func (s *cleanSuite) TestFailDeleteRRSet() {
	s.setupCommonMocks()
	rrset := stackitdnsclient_new.RecordSet{
		Id: toPtr("1234"),
	}
	s.mockRRSetRepository.EXPECT().
		FetchRRSetForZone(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&rrset, nil)
	s.mockRRSetRepository.EXPECT().
		DeleteRRSet(gomock.Any(), *rrset.Id).
		Return(fmt.Errorf("error deleting rr set"))

	err := s.resolver.CleanUp(challengeRequest)
	s.Error(err)
	s.Containsf(
		err.Error(),
		"error deleting rr set",
		"error message should contain error from rrSetRepository",
	)
}

func (s *cleanSuite) TestSuccessDeleteRRSet() {
	s.setupCommonMocks()
	rrset := stackitdnsclient_new.RecordSet{
		Id: toPtr("1234"),
	}
	s.mockRRSetRepository.EXPECT().
		FetchRRSetForZone(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&rrset, nil)
	s.mockRRSetRepository.EXPECT().
		DeleteRRSet(gomock.Any(), *rrset.Id).
		Return(nil)

	err := s.resolver.CleanUp(challengeRequest)
	s.NoError(err)
}

func toPtr(str string) *string {
	return &str
}
