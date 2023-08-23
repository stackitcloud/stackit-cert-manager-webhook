package repository

import (
	"context"
	"fmt"

	"github.com/antihax/optional"
	stackitdnsclient "github.com/stackitcloud/stackit-dns-api-client-go"
)

var ErrRRSetNotFound = fmt.Errorf("rrset not found")

//go:generate mockgen -destination=./mock/rrset_repository.go -source=./rrset_repository.go RRSetRepository
type RRSetRepository interface {
	FetchRRSetForZone(ctx context.Context, rrSetName string, rrSetType string) (*stackitdnsclient.DomainRrSet, error)
	CreateRRSet(ctx context.Context, rrSet stackitdnsclient.RrsetRrSetPost) error
	UpdateRRSet(ctx context.Context, rrSet stackitdnsclient.DomainRrSet) error
	DeleteRRSet(ctx context.Context, rrSetId string) error
}

//go:generate mockgen -destination=./mock/rrset_repository.go -source=./rrset_repository.go RRSetRepositoryFactory
type RRSetRepositoryFactory interface {
	NewRRSetRepository(config Config, zoneId string) RRSetRepository
}

type rrSetRepository struct {
	apiClient *stackitdnsclient.APIClient
	projectId string
	zoneId    string
}

type rrSetRepositoryFactory struct{}

func (r rrSetRepositoryFactory) NewRRSetRepository(
	config Config,
	zoneId string,
) RRSetRepository {
	apiClient := newStackitDnsClient(config)

	return &rrSetRepository{
		apiClient: apiClient,
		projectId: config.ProjectId,
		zoneId:    zoneId,
	}
}

func NewRRSetRepositoryFactory() RRSetRepositoryFactory {
	return rrSetRepositoryFactory{}
}

// FetchRRSetForZone fetch specific rr set for a zone.
func (r *rrSetRepository) FetchRRSetForZone(
	ctx context.Context,
	rrSetName string,
	rrSetType string,
) (*stackitdnsclient.DomainRrSet, error) {
	queryParams := stackitdnsclient.RecordSetApiV1ProjectsProjectIdZonesZoneIdRrsetsGetOpts{
		ActiveEq: optional.NewBool(true),
		NameEq:   optional.NewString(rrSetName),
		TypeEq:   optional.NewString(rrSetType),
	}

	rrSetResponse, _, err := r.apiClient.RecordSetApi.V1ProjectsProjectIdZonesZoneIdRrsetsGet(
		ctx,
		r.projectId,
		r.zoneId,
		&queryParams,
	)
	if err != nil {
		return nil, err
	}

	if len(rrSetResponse.RrSets) == 0 {
		return nil, ErrRRSetNotFound
	}

	return &rrSetResponse.RrSets[0], nil
}

func (r *rrSetRepository) CreateRRSet(
	ctx context.Context,
	rrSet stackitdnsclient.RrsetRrSetPost,
) error {
	_, _, err := r.apiClient.RecordSetApi.V1ProjectsProjectIdZonesZoneIdRrsetsPost(
		ctx,
		rrSet,
		r.projectId,
		r.zoneId,
	)
	if err != nil {
		return err
	}

	return nil
}

func (r *rrSetRepository) UpdateRRSet(
	ctx context.Context,
	rrSet stackitdnsclient.DomainRrSet,
) error {
	records := make([]stackitdnsclient.RrsetRecordPost, len(rrSet.Records))
	for i, record := range rrSet.Records {
		records[i] = stackitdnsclient.RrsetRecordPost{
			Content: record.Content,
		}
	}
	rrSetBody := stackitdnsclient.RrsetRrSetPatch{
		Comment: rrSet.Comment,
		Name:    rrSet.Name,
		Records: records,
		Ttl:     rrSet.Ttl,
	}

	_, _, err := r.apiClient.RecordSetApi.V1ProjectsProjectIdZonesZoneIdRrsetsRrSetIdPatch(
		ctx,
		rrSetBody,
		r.projectId,
		r.zoneId,
		rrSet.Id,
	)
	if err != nil {
		return err
	}

	return nil
}

func (r *rrSetRepository) DeleteRRSet(ctx context.Context, rrSetId string) error {
	_, resp, err := r.apiClient.RecordSetApi.V1ProjectsProjectIdZonesZoneIdRrsetsRrSetIdDelete(
		ctx,
		r.projectId,
		r.zoneId,
		rrSetId,
	)
	if resp != nil {
		if resp.StatusCode == 404 || resp.StatusCode == 400 {
			return ErrRRSetNotFound
		}
	}

	return err
}
