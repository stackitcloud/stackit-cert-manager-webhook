package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/antihax/optional"
	stackitdnsclient "github.com/stackitcloud/stackit-dns-api-client-go"
)

var ErrZoneNotFound = fmt.Errorf("zone not found")

//go:generate mockgen -destination=./mock/zone_repository.go -source=./zone_repository.go ZoneRepository
type ZoneRepository interface {
	FetchZone(ctx context.Context, zoneDnsName string) (*stackitdnsclient.DomainZone, error)
}

//go:generate mockgen -destination=./mock/zone_repository.go -source=./zone_repository.go ZoneRepositoryFactory
type ZoneRepositoryFactory interface {
	NewZoneRepository(config Config) ZoneRepository
}

type zoneRepository struct {
	apiClient *stackitdnsclient.APIClient
	projectId string
}

type zoneRepositoryFactory struct{}

func (z zoneRepositoryFactory) NewZoneRepository(
	config Config,
) ZoneRepository {
	apiClient := newStackitDnsClient(config)

	return &zoneRepository{
		apiClient: apiClient,
		projectId: config.ProjectId,
	}
}

func NewZoneRepositoryFactory() ZoneRepositoryFactory {
	return zoneRepositoryFactory{}
}

func (z *zoneRepository) FetchZone(
	ctx context.Context,
	zoneDnsName string,
) (*stackitdnsclient.DomainZone, error) {
	queryParams := stackitdnsclient.ZoneApiV1ProjectsProjectIdZonesGetOpts{
		ActiveEq:  optional.NewBool(true),
		DnsNameEq: optional.NewString(strings.ToLower(zoneDnsName)),
	}

	zoneResponse, _, err := z.apiClient.ZoneApi.V1ProjectsProjectIdZonesGet(
		ctx,
		z.projectId,
		&queryParams,
	)
	if err != nil {
		return nil, err
	}

	if len(zoneResponse.Zones) == 0 {
		return nil, ErrZoneNotFound
	}

	return &zoneResponse.Zones[0], nil
}
