package repository

import (
	"context"
	"fmt"

	stackitdnsclient "github.com/stackitcloud/stackit-sdk-go/services/dns"
)

var ErrRRSetNotFound = fmt.Errorf("rrset not found")

//go:generate mockgen -destination=./mock/rrset_repository.go -source=./rrset_repository.go RRSetRepository
type RRSetRepository interface {
	FetchRRSetForZone(ctx context.Context, rrSetName string, rrSetType string) (*stackitdnsclient.RecordSet, error)
	CreateRRSet(ctx context.Context, rrSet stackitdnsclient.RecordSet) error
	UpdateRRSet(ctx context.Context, rrSet stackitdnsclient.RecordSet) error
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
	apiClient, _ := newStackitDnsClient() // todo add fitting config

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
) (*stackitdnsclient.RecordSet, error) {
	var pager int32 = 1
	listRequest := r.apiClient.ListRecordSets(ctx, r.projectId, r.zoneId).
		Page(pager).PageSize(10000).
		ActiveEq(true).NameEq(rrSetName).TypeEq(rrSetType)

	rrSetResponse, err := listRequest.Execute()
	if err != nil {
		return nil, err
	}

	if len(*rrSetResponse.RrSets) == 0 {
		return nil, ErrRRSetNotFound
	}

	return &(*rrSetResponse.RrSets)[0], nil
}

func (r *rrSetRepository) CreateRRSet(
	ctx context.Context,
	rrSet stackitdnsclient.RecordSet,
) error {
	records := make([]stackitdnsclient.RecordPayload, len(*rrSet.Records))
	for i, record := range *rrSet.Records {
		records[i] = stackitdnsclient.RecordPayload{
			Content: record.Content,
		}
	}
	payload := stackitdnsclient.CreateRecordSetPayload{
		Comment: rrSet.Comment,
		Name:    rrSet.Name,
		Ttl:     rrSet.Ttl,
		Type:    rrSet.Type,
	}
	_, err := r.apiClient.CreateRecordSet(ctx, r.projectId, r.zoneId).CreateRecordSetPayload(payload).Execute()
	if err != nil {
		return err
	}

	return nil
}

func (r *rrSetRepository) UpdateRRSet(
	ctx context.Context,
	rrSet stackitdnsclient.RecordSet,
) error {
	records := make([]stackitdnsclient.RecordPayload, len(*rrSet.Records))
	for i, record := range *rrSet.Records {
		records[i] = stackitdnsclient.RecordPayload{
			Content: record.Content,
		}
	}
	payload := stackitdnsclient.PartialUpdateRecordSetPayload{
		Comment: rrSet.Comment,
		Name:    rrSet.Name,
		Records: &records,
		Ttl:     rrSet.Ttl,
	}

	_, err := r.apiClient.PartialUpdateRecordSet(ctx, r.projectId, r.zoneId, *rrSet.Id).
		PartialUpdateRecordSetPayload(payload).Execute()
	if err != nil {
		return err
	}

	return nil
}

func (r *rrSetRepository) DeleteRRSet(ctx context.Context, rrSetId string) error {
	_, err := r.apiClient.DeleteRecordSet(ctx, r.projectId, r.zoneId, rrSetId).Execute()

	// maybe check response here later
	// if resp != nil {
	// 	if resp.Message == 404 || resp.StatusCode == 400 {
	// 		return ErrRRSetNotFound
	// 	}
	// }

	return err
}
