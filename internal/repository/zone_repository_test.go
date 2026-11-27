package repository_test

import (
	"context"
	"testing"

	"github.com/stackitcloud/stackit-cert-manager-webhook/internal/repository"
	stackitconfig "github.com/stackitcloud/stackit-sdk-go/core/config"
	stackitdnsclient "github.com/stackitcloud/stackit-sdk-go/services/dns/v1api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestZoneRepository_FetchZone(t *testing.T) {
	t.Parallel()

	ctx := context.TODO()
	server := getTestServer(t)
	t.Cleanup(server.Close)

	apiClient, _ := stackitdnsclient.NewAPIClient(
		stackitconfig.WithEndpoint(server.URL),
		stackitconfig.WithHTTPClient(server.Client()),
		stackitconfig.WithoutAuthentication(),
	)

	createZoneRepo := func(projectID string) repository.ZoneRepository {
		config := repository.Config{
			ProjectId: projectID,
			ApiClient: apiClient,
		}
		zoneRepository, err := repository.NewZoneRepositoryFactory().NewZoneRepository(config)
		require.NoError(t, err)

		return zoneRepository
	}

	testCases := []struct {
		name        string
		projectID   string
		expectErr   bool
		specificErr error
		expectedID  string
	}{
		{"success valid ID", "1234", false, nil, "1234"},
		{"failure invalid ID", "5678", true, nil, ""},
		{"failure zone not found", "0000", true, repository.ErrZoneNotFound, ""},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			zoneRepository := createZoneRepo(tc.projectID)
			zone, err := zoneRepository.FetchZone(ctx, "test-zone")

			if tc.expectErr {
				assert.Error(t, err)
				if tc.specificErr != nil {
					assert.ErrorIs(t, err, tc.specificErr)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedID, zone.Id)
			}
		})
	}
}
