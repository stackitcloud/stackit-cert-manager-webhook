package repository

import (
	"context"
	"fmt"
	"strings"

	stackitdnsclient "github.com/stackitcloud/stackit-sdk-go/services/dns"
)

var ErrZoneNotFound = fmt.Errorf("zone not found")

//go:generate mockgen -destination=./mock/zone_repository.go -source=./zone_repository.go ZoneRepository
type ZoneRepository interface {
	FetchZone(ctx context.Context, zoneDnsName string) (*stackitdnsclient.Zone, error)
}

//go:generate mockgen -destination=./mock/zone_repository.go -source=./zone_repository.go ZoneRepositoryFactory
type ZoneRepositoryFactory interface {
	NewZoneRepository(config Config) (ZoneRepository, error)
}

type zoneRepository struct {
	apiClient *stackitdnsclient.APIClient
	projectId string
}

type zoneRepositoryFactory struct{}

func (z zoneRepositoryFactory) NewZoneRepository(
	config Config,
) (ZoneRepository, error) {
	apiClient, err := chooseNewStackitDnsClient(config)
	if err != nil {
		return nil, err
	}

	return &zoneRepository{
		apiClient: apiClient,
		projectId: config.ProjectId,
	}, nil
}

func NewZoneRepositoryFactory() ZoneRepositoryFactory {
	return zoneRepositoryFactory{}
}

func (z *zoneRepository) FetchZone(
	ctx context.Context,
	zoneDnsName string,
) (*stackitdnsclient.Zone, error) {
	zoneResponse, err := z.apiClient.ListZones(ctx, z.projectId).ActiveEq(true).DnsNameEq(strings.ToLower(zoneDnsName)).Execute()
	if err != nil {
		return nil, err
	}

	if len(*zoneResponse.Zones) == 0 {
		return nil, ErrZoneNotFound
	}

	return &(*zoneResponse.Zones)[0], nil
}
