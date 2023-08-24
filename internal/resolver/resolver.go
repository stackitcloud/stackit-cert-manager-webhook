package resolver

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/cert-manager/cert-manager/pkg/acme/webhook"
	"github.com/cert-manager/cert-manager/pkg/acme/webhook/apis/acme/v1alpha1"
	"github.com/stackitcloud/stackit-cert-manager-webhook/internal/repository"
	stackitdnsclient "github.com/stackitcloud/stackit-dns-api-client-go"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const typeTxtRecord = "TXT"

func NewResolver(
	httpClient *http.Client,
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
	}
}

type stackitDnsProviderResolver struct {
	ctx                    context.Context
	httpClient             *http.Client
	configProvider         ConfigProvider
	secretFetcher          SecretFetcher
	zoneRepositoryFactory  repository.ZoneRepositoryFactory
	rrSetRepositoryFactory repository.RRSetRepositoryFactory
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
	rrSetRepository, rrSetName, err := s.initializeResolverContext(ch)
	if err != nil {
		return err
	}

	rrSet, err := rrSetRepository.FetchRRSetForZone(s.ctx, rrSetName, typeTxtRecord)
	if errors.Is(err, repository.ErrRRSetNotFound) {
		return s.createRRSet(rrSetRepository, rrSetName, ch.Key)
	} else if err != nil {
		return err
	}

	return rrSetRepository.UpdateRRSet(s.ctx, *rrSet)
}

// CleanUp should delete the relevant TXT record from the DNS provider console.
// If multiple TXT records exist with the same record name (e.g.
// _acme-challenge.example.com) then **only** the record with the same `key`
// value provided on the ChallengeRequest should be cleaned up.
// This is in order to facilitate multiple DNS validations for the same domain
// concurrently.
func (s *stackitDnsProviderResolver) CleanUp(ch *v1alpha1.ChallengeRequest) error { //nolint:gocognit // clean enough
	rrSetRepository, rrSetName, err := s.initializeResolverContext(ch)
	if err != nil && errors.Is(err, repository.ErrZoneNotFound) {
		return nil
	}
	if err != nil {
		return err
	}

	rrSet, err := rrSetRepository.FetchRRSetForZone(s.ctx, rrSetName, typeTxtRecord)
	// if the rr set does not exist it may be already deleted
	if err != nil && errors.Is(err, repository.ErrRRSetNotFound) {
		return nil
	}
	if err != nil {
		return err
	}

	err = rrSetRepository.DeleteRRSet(s.ctx, rrSet.Id)
	if err != nil && errors.Is(err, repository.ErrRRSetNotFound) {
		return nil
	}
	if err != nil {
		return err
	}

	return nil
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
	cl, err := kubernetes.NewForConfig(kubeClientConfig)
	if err != nil {
		return err
	}

	s.secretFetcher = &kubeSecretFetcher{
		client: cl,
		ctx:    s.ctx,
	}

	return nil
}

func (s *stackitDnsProviderResolver) initializeResolverContext(
	ch *v1alpha1.ChallengeRequest,
) (repository.RRSetRepository, string, error) {
	cfg, err := s.configProvider.LoadConfig(ch.Config)
	if err != nil {
		return nil, "", err
	}

	authToken, err := s.getAuthToken(&cfg)
	if err != nil {
		return nil, "", err
	}

	config := s.getRepositoryConfig(cfg, authToken)

	zoneDnsName, rrSetName := getZoneDnsNameAndRRSetName(ch)
	zoneRepository := s.zoneRepositoryFactory.NewZoneRepository(config)
	zone, err := zoneRepository.FetchZone(s.ctx, zoneDnsName)
	if err != nil {
		return nil, "", err
	}

	rrSetRepository := s.rrSetRepositoryFactory.NewRRSetRepository(config, zone.Id)

	return rrSetRepository, rrSetName, nil
}

func (s *stackitDnsProviderResolver) getRepositoryConfig(
	cfg StackitDnsProviderConfig,
	authToken string,
) repository.Config {
	config := repository.Config{
		ApiBasePath: cfg.ApiBasePath,
		AuthToken:   authToken,
		ProjectId:   cfg.ProjectId,
		HttpClient:  s.httpClient,
	}

	return config
}

func (s *stackitDnsProviderResolver) createRRSet(
	repo repository.RRSetRepository,
	rrSetName, key string,
) error {
	rrSet := stackitdnsclient.RrsetRrSetPost{
		Comment: "This record set is managed by stackit-cert-manager-webhook",
		Name:    rrSetName,
		Records: []stackitdnsclient.RrsetRecordPost{
			{
				Content: key,
			},
		},
		Ttl:   60,
		Type_: typeTxtRecord,
	}

	return repo.CreateRRSet(s.ctx, rrSet)
}

// getAuthToken from Kubernetes secretFetcher.
func (s *stackitDnsProviderResolver) getAuthToken(cfg *StackitDnsProviderConfig) (string, error) {
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

func getZoneDnsNameAndRRSetName(ch *v1alpha1.ChallengeRequest) (string, string) {
	// Remove trailing . from domain
	domain := strings.TrimSuffix(ch.ResolvedZone, ".")

	return domain, ch.ResolvedFQDN
}
