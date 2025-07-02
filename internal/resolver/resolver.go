package resolver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/cert-manager/cert-manager/pkg/acme/webhook"
	"github.com/cert-manager/cert-manager/pkg/acme/webhook/apis/acme/v1alpha1"
	"github.com/stackitcloud/stackit-cert-manager-webhook/internal/repository"
	stackitdnsclient "github.com/stackitcloud/stackit-sdk-go/services/dns"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const typeTxtRecord = "TXT"

var stackitAuthToken = os.Getenv("STACKIT_AUTH_TOKEN")

func NewResolver(
	httpClient *http.Client,
	logger *zap.Logger,
	zoneRepositoryFactory repository.ZoneRepositoryFactory,
	rrSetRepositoryFactory repository.RRSetRepositoryFactory,
	secretFetcher SecretFetcher,
	configProvider ConfigProvider,
) webhook.Solver {
	return &stackitDnsProviderResolver{
		ctx:                    context.Background(),
		httpClient:             httpClient,
		configProvider:         configProvider,
		secretFetcher:          secretFetcher,
		zoneRepositoryFactory:  zoneRepositoryFactory,
		rrSetRepositoryFactory: rrSetRepositoryFactory,
		logger:                 logger,
	}
}

type stackitDnsProviderResolver struct {
	ctx                    context.Context
	httpClient             *http.Client
	configProvider         ConfigProvider
	secretFetcher          SecretFetcher
	zoneRepositoryFactory  repository.ZoneRepositoryFactory
	rrSetRepositoryFactory repository.RRSetRepositoryFactory
	logger                 *zap.Logger
}

// Name is used as the name for this DNS solver when referencing it on the ACME
// Issuer resource.
// This should be unique **within the group name**, i.e. you can have two
// solvers configured with the same Name() **so long as they do not co-exist
// within a single webhook deployment**.
// For example, `cloudflare` may be used as the name of a solver.
func (s *stackitDnsProviderResolver) Name() string {
	return "stackit"
}

// Present is responsible for actually presenting the DNS record with the
// DNS provider.
// This method should tolerate being called multiple times with the same value.
// cert-manager itself will later perform a self check to ensure that the
// solver has correctly configured the DNS provider.
func (s *stackitDnsProviderResolver) Present(ch *v1alpha1.ChallengeRequest) error {
	initResolverRes, err := s.initializeResolverContext(ch)
	if err != nil {
		return err
	}

	rrSet, err := initResolverRes.rrSetRepository.FetchRRSetForZone(
		s.ctx,
		initResolverRes.rrSetName,
		typeTxtRecord,
	)
	if errors.Is(err, repository.ErrRRSetNotFound) {
		return s.handleRRSetNotFound(initResolverRes, ch.Key)
	} else if err != nil {
		return err
	}

	return s.updateExistingRRSet(initResolverRes, rrSet)
}

// CleanUp should delete the relevant TXT record from the DNS provider console.
// If multiple TXT records exist with the same record name (e.g.
// _acme-challenge.example.com) then **only** the record with the same `key`
// value provided on the ChallengeRequest should be cleaned up.
// This is in order to facilitate multiple DNS validations for the same domain
// concurrently.
func (s *stackitDnsProviderResolver) CleanUp(ch *v1alpha1.ChallengeRequest) error {
	initResolverRes, err := s.initializeResolverContext(ch)
	if err != nil {
		return s.handleErrorDuringInitialization(err)
	}

	return s.handleRRSetCleanup(initResolverRes)
}

// Initialize will be called when the webhook first starts.
// This method can be used to instantiate the webhook, i.e. initializing
// connections or warming up caches.
// Typically, the kubeClientConfig parameter is used to build a Kubernetes
// client that can be used to fetch resources from the Kubernetes API, e.g.
// Secret resources containing credentials used to authenticate with DNS
// provider accounts.
// The stopCh can be used to handle early termination of the webhook, in cases
// where a SIGTERM or similar signal is sent to the webhook process.
func (s *stackitDnsProviderResolver) Initialize(
	kubeClientConfig *rest.Config,
	stopCh <-chan struct{},
) error {
	s.logger.Info("Initializing stackit resolver")

	cl, err := kubernetes.NewForConfig(kubeClientConfig)
	if err != nil {
		s.logger.Error("Error initializing kubernetes client", zap.Error(err))

		return err
	}

	s.secretFetcher = &kubeSecretFetcher{
		client: cl,
		ctx:    s.ctx,
	}

	s.logger.Info("Stackit resolver initialized")

	return nil
}

func (s *stackitDnsProviderResolver) initializeResolverContext(
	ch *v1alpha1.ChallengeRequest,
) (*initResolverContextResult, error) {
	cfg, err := s.configProvider.LoadConfig(ch.Config)
	if err != nil {
		return nil, err
	}

	config, err := s.getRepositoryConfig(&cfg)
	if err != nil {
		return nil, err
	}

	zoneDnsName, rrSetName := getZoneDnsNameAndRRSetName(ch)
	zoneRepository, err := s.zoneRepositoryFactory.NewZoneRepository(config)
	if err != nil {
		s.logger.Error("Error creating zone repository", zap.Error(err))

		return nil, err
	}

	s.logger.Info("Fetching zone", zap.String("zoneDnsName", zoneDnsName))

	zone, err := zoneRepository.FetchZone(s.ctx, zoneDnsName)
	if err != nil {
		s.logger.Error(
			"Error fetching zone",
			zap.Error(err),
			zap.String("zoneDnsName", zoneDnsName),
		)

		return nil, err
	}

	s.logger.Info("Zone fetched", zap.String("zoneDnsName", zoneDnsName))

	rrSetRepository, err := s.rrSetRepositoryFactory.NewRRSetRepository(config, *zone.Id)
	if err != nil {
		s.logger.Error("Error creating RRSet repository", zap.Error(err))

		return nil, err
	}

	return &initResolverContextResult{
		rrSetRepository:   rrSetRepository,
		rrSetName:         rrSetName,
		acmeTxtDefaultTTL: cfg.AcmeTxtRecordTTL,
	}, nil
}

func (s *stackitDnsProviderResolver) createRRSet(
	initResolverRes *initResolverContextResult, key string,
) error {
	comment := "This record set is managed by stackit-cert-manager-webhook"
	rrSetType := typeTxtRecord

	rrSet := stackitdnsclient.RecordSet{
		Comment: &comment,
		Name:    &initResolverRes.rrSetName,
		Records: &[]stackitdnsclient.Record{
			{
				Content: &key,
			},
		},
		Ttl:  &initResolverRes.acmeTxtDefaultTTL,
		Type: stackitdnsclient.RecordSetGetTypeAttributeType(&rrSetType),
	}

	s.logger.Info("Creating RRSet", zap.String("rrSet", fmt.Sprintf("%+v", rrSet)))

	return initResolverRes.rrSetRepository.CreateRRSet(s.ctx, rrSet)
}

// getAuthToken from Kubernetes secretFetcher.
func (s *stackitDnsProviderResolver) getAuthToken(cfg *StackitDnsProviderConfig) (string, error) {
	if stackitAuthToken != "" {
		return stackitAuthToken, nil
	}

	token, err := s.secretFetcher.StringFromSecret(
		cfg.AuthTokenSecretNamespace,
		cfg.AuthTokenSecretRef,
		cfg.AuthTokenSecretKey,
	)
	if err != nil {
		return "", err
	}

	return token, nil
}

// geSaKeyPath gets the Service Account Key Path from the environment.
func (s *stackitDnsProviderResolver) getSaKeyPath(cfg *StackitDnsProviderConfig) string {
	if cfg.ServiceAccountKeyPath != "" {
		return cfg.ServiceAccountKeyPath
	}

	return os.Getenv("STACKIT_SERVICE_ACCOUNT_KEY_PATH")
}

func (s *stackitDnsProviderResolver) checkUseSaAuthentication(cfg *StackitDnsProviderConfig) bool {
	return s.getSaKeyPath(cfg) != ""
}

func (s *stackitDnsProviderResolver) getRepositoryConfig(
	cfg *StackitDnsProviderConfig,
) (repository.Config, error) {
	config := repository.Config{
		ApiBasePath: cfg.ApiBasePath,
		ProjectId:   cfg.ProjectId,
		HttpClient:  s.httpClient,
		UseSaKey:    false,
	}

	switch {
	case s.checkUseSaAuthentication(cfg):
		config.SaKeyPath = s.getSaKeyPath(cfg)
		config.UseSaKey = true
		s.logger.Info(
			"Using service account key for authentication",
			zap.String("saKeyPath", config.SaKeyPath),
		)
	default:
		authToken, err := s.getAuthToken(cfg)
		if err != nil {
			return repository.Config{}, err
		}
		config.AuthToken = authToken
		s.logger.Info("Using auth token for authentication")
	}

	return config, nil
}

func getZoneDnsNameAndRRSetName(ch *v1alpha1.ChallengeRequest) (string, string) {
	// Remove trailing . from domain
	domain := strings.TrimSuffix(ch.ResolvedZone, ".")

	return domain, ch.ResolvedFQDN
}

func (s *stackitDnsProviderResolver) handleErrorDuringInitialization(
	err error,
) error {
	if errors.Is(err, repository.ErrZoneNotFound) {
		return nil
	}

	return err
}

func (s *stackitDnsProviderResolver) handleRRSetCleanup(
	initResolverRes *initResolverContextResult,
) error {
	s.logger.Info("Cleaning up RRSet", zap.String("rrSetName", initResolverRes.rrSetName))

	rrSet, err := initResolverRes.rrSetRepository.FetchRRSetForZone(
		s.ctx,
		initResolverRes.rrSetName,
		typeTxtRecord,
	)
	if err != nil {
		return s.handleFetchRRSetError(err, initResolverRes.rrSetName)
	}

	return s.deleteRRSet(initResolverRes.rrSetRepository, rrSet, initResolverRes.rrSetName)
}

func (s *stackitDnsProviderResolver) handleFetchRRSetError(err error, rrSetName string) error {
	if errors.Is(err, repository.ErrRRSetNotFound) {
		s.logger.Info("RRSet not found, nothing to clean up", zap.String("rrSetName", rrSetName))

		return nil
	}

	s.logger.Error("Error fetching RRSet", zap.Error(err), zap.String("rrSetName", rrSetName))

	return err
}

func (s *stackitDnsProviderResolver) deleteRRSet(
	rrSetRepository repository.RRSetRepository,
	rrSet *stackitdnsclient.RecordSet,
	rrSetName string,
) error {
	if rrSet == nil {
		return nil
	}
	err := rrSetRepository.DeleteRRSet(s.ctx, *rrSet.Id)
	if err != nil {
		return s.handleDeleteRRSetError(err, rrSetName, *rrSet.Id)
	}

	s.logger.Info(
		"RRSet deleted",
		zap.String("rrSetName", rrSetName),
		zap.String("rrSetId", *rrSet.Id),
	)

	return nil
}

func (s *stackitDnsProviderResolver) handleDeleteRRSetError(
	err error,
	rrSetName, rrSetId string,
) error {
	if errors.Is(err, repository.ErrRRSetNotFound) {
		s.logger.Info(
			"RRSet not found, nothing to clean up",
			zap.String("rrSetName", rrSetName),
			zap.String("rrSetId", rrSetId),
		)

		return nil
	}

	s.logger.Error(
		"Error deleting RRSet",
		zap.Error(err),
		zap.String("rrSetName", rrSetName),
		zap.String("rrSetId", rrSetId),
	)

	return err
}

func (s *stackitDnsProviderResolver) handleRRSetNotFound(
	initResolverRes *initResolverContextResult,
	challengeKey string,
) error {
	s.logger.Info(
		"RRSet not found, creating new RRSet",
		zap.String("rrSetName", initResolverRes.rrSetName),
	)

	if err := s.createRRSet(initResolverRes, challengeKey); err != nil {
		s.logger.Error(
			"Error creating RRSet",
			zap.Error(err),
			zap.String("rrSetName", initResolverRes.rrSetName),
		)

		return err
	}

	s.logger.Info("RRSet created", zap.String("rrSetName", initResolverRes.rrSetName))

	return nil
}

func (s *stackitDnsProviderResolver) updateExistingRRSet(
	initResolverRes *initResolverContextResult,
	rrSet *stackitdnsclient.RecordSet,
) error {
	s.logger.Info("RRSet found, updating RRSet", zap.String("rrSetName", initResolverRes.rrSetName))

	rrSet.Ttl = &initResolverRes.acmeTxtDefaultTTL

	if err := initResolverRes.rrSetRepository.UpdateRRSet(s.ctx, *rrSet); err != nil {
		s.logger.Error(
			"Error updating RRSet",
			zap.Error(err),
			zap.String("rrSetName", initResolverRes.rrSetName),
		)

		return err
	}

	s.logger.Info("RRSet updated", zap.String("rrSetName", initResolverRes.rrSetName))

	return nil
}

type initResolverContextResult struct {
	rrSetRepository   repository.RRSetRepository
	rrSetName         string
	acmeTxtDefaultTTL int64
}
