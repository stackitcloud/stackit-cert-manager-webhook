package repository_test

import (
	"context"
	"testing"

	"github.com/stackitcloud/stackit-cert-manager-webhook/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//nolint:tparallel // sdk has data races in parallel testing
func TestZoneRepository_FetchZone(t *testing.T) {
	t.Parallel()

	ctx := context.TODO()
	server := getTestServer(t)
	t.Cleanup(server.Close)

	createZoneRepo := func(projectID string) repository.ZoneRepository {
		config := repository.Config{
			ApiBasePath: server.URL,
			AuthToken:   "test-token",
			ProjectId:   projectID,
			HttpClient:  server.Client(),
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

	//nolint:paralleltest // sdk has data races in parallel testing
	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			zoneRepository := createZoneRepo(tc.projectID)
			zone, err := zoneRepository.FetchZone(ctx, "test-zone")

			if tc.expectErr {
				assert.Error(t, err)
				if tc.specificErr != nil {
					assert.ErrorIs(t, err, tc.specificErr)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedID, *zone.Id)
			}
		})
	}
}
