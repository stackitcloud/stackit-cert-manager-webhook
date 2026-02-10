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
		Return(&stackitdnsclient_new.RecordSet{
			Records: &[]stackitdnsclient_new.Record{},
		}, nil)
	s.mockRRSetRepository.EXPECT().
		UpdateRRSet(gomock.Any(), matchedBy(func(rrSet stackitdnsclient_new.RecordSet) bool {
			return rrSet.Records != nil && len(*rrSet.Records) == 1
		})).
		Return(nil)

	err := s.resolver.Present(challengeRequest)
	s.NoError(err)
}

func (s *presentSuite) TestSuccessPresentIdempotent() {
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

	challengeKey := "challenge-key"
	req := &v1alpha1.ChallengeRequest{
		Config: configJson,
		Key:    challengeKey,
	}

	s.mockRRSetRepository.EXPECT().
		FetchRRSetForZone(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&stackitdnsclient_new.RecordSet{
			Records: &[]stackitdnsclient_new.Record{
				{Content: &challengeKey},
			},
		}, nil)

	s.mockRRSetRepository.EXPECT().
		UpdateRRSet(gomock.Any(), matchedBy(func(rrSet stackitdnsclient_new.RecordSet) bool {
			return len(*rrSet.Records) == 1 && *(*rrSet.Records)[0].Content == challengeKey
		})).
		Return(nil)

	err := s.resolver.Present(req)
	s.NoError(err)
}

//nolint:gocognit // this is a test
func (s *presentSuite) TestSuccessPresentAppended() {
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

	existingKey := "existing-key"
	newKey := "new-key"
	req := &v1alpha1.ChallengeRequest{
		Config: configJson,
		Key:    newKey,
	}

	s.mockRRSetRepository.EXPECT().
		FetchRRSetForZone(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&stackitdnsclient_new.RecordSet{
			Records: &[]stackitdnsclient_new.Record{
				{Content: &existingKey},
			},
		}, nil)

	s.mockRRSetRepository.EXPECT().
		UpdateRRSet(gomock.Any(), matchedBy(func(rrSet stackitdnsclient_new.RecordSet) bool {
			if rrSet.Records == nil || len(*rrSet.Records) != 2 {
				return false
			}
			foundExisting := false
			foundNew := false
			for _, r := range *rrSet.Records {
				if r.Content != nil && *r.Content == existingKey {
					foundExisting = true
				}
				if r.Content != nil && *r.Content == newKey {
					foundNew = true
				}
			}

			return foundExisting && foundNew
		})).
		Return(nil)

	err := s.resolver.Present(req)
	s.NoError(err)
}

func (s *presentSuite) TestPresentRRSetWithEmptyRecords() {
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
		Return(&stackitdnsclient_new.RecordSet{
			Records: &[]stackitdnsclient_new.Record{},
		}, nil)
	s.mockRRSetRepository.EXPECT().
		UpdateRRSet(gomock.Any(), matchedBy(func(rrSet stackitdnsclient_new.RecordSet) bool {
			return len(*rrSet.Records) == 1
		})).Return(nil)
	err := s.resolver.Present(challengeRequest)
	s.NoError(err)
}

func (s *presentSuite) TestFailCreateRRSet() {
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
		Return(fmt.Errorf("error creating rr set"))

	err := s.resolver.Present(challengeRequest)
	s.Error(err)
	s.Contains(err.Error(), "error creating rr set")
}

func (s *presentSuite) TestFailUpdateRRSet() {
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
		Return(&stackitdnsclient_new.RecordSet{
			Records: &[]stackitdnsclient_new.Record{},
		}, nil)
	s.mockRRSetRepository.EXPECT().
		UpdateRRSet(gomock.Any(), gomock.Any()).
		Return(fmt.Errorf("error updating rr set"))

	err := s.resolver.Present(challengeRequest)
	s.Error(err)
	s.Contains(err.Error(), "error updating rr set")
}

func (s *presentSuite) TestTTLPropagation() {
	ttl := int64(600)
	// Test Create
	s.mockConfigProvider.EXPECT().
		LoadConfig(gomock.Any()).
		Return(resolver.StackitDnsProviderConfig{AcmeTxtRecordTTL: ttl}, nil)
	s.mockSecretFetcher.EXPECT().
		StringFromSecret(gomock.Any(), gomock.Any(), gomock.Any()).
		Return("", nil).AnyTimes()
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
		CreateRRSet(gomock.Any(), matchedBy(func(rrSet stackitdnsclient_new.RecordSet) bool {
			return *rrSet.Ttl == ttl
		})).
		Return(nil)

	err := s.resolver.Present(challengeRequest)
	s.NoError(err)

	// Test Update
	s.mockConfigProvider.EXPECT().
		LoadConfig(gomock.Any()).
		Return(resolver.StackitDnsProviderConfig{AcmeTxtRecordTTL: ttl}, nil)
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
		Return(&stackitdnsclient_new.RecordSet{
			Records: &[]stackitdnsclient_new.Record{},
		}, nil)
	s.mockRRSetRepository.EXPECT().
		UpdateRRSet(gomock.Any(), matchedBy(func(rrSet stackitdnsclient_new.RecordSet) bool {
			return *rrSet.Ttl == ttl
		})).
		Return(nil)

	err = s.resolver.Present(challengeRequest)
	s.NoError(err)
}

func (s *presentSuite) TestAuthMethodSelection() {
	// Test Service Account
	s.Run("Service Account", func() {
		s.mockConfigProvider.EXPECT().
			LoadConfig(gomock.Any()).
			Return(resolver.StackitDnsProviderConfig{
				ServiceAccountKeyPath: "/path/to/key",
			}, nil)
		s.mockZoneRepositoryFactory.EXPECT().
			NewZoneRepository(matchedBy(func(cfg repository.Config) bool {
				return cfg.UseSaKey && cfg.SaKeyPath == "/path/to/key"
			})).
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
	})

	// Test Auth Token
	s.Run("Auth Token", func() {
		s.mockConfigProvider.EXPECT().
			LoadConfig(gomock.Any()).
			Return(resolver.StackitDnsProviderConfig{
				AuthTokenSecretRef: "secret",
			}, nil)
		s.mockSecretFetcher.EXPECT().
			StringFromSecret(gomock.Any(), gomock.Any(), gomock.Any()).
			Return("token123", nil)
		s.mockZoneRepositoryFactory.EXPECT().
			NewZoneRepository(matchedBy(func(cfg repository.Config) bool {
				return !cfg.UseSaKey && cfg.AuthToken == "token123"
			})).
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
	})
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
