package repository_test

import (
	"context"
	"testing"

	"github.com/stackitcloud/stackit-cert-manager-webhook/internal/repository"
	stackitdnsclient "github.com/stackitcloud/stackit-sdk-go/services/dns"

	"github.com/stretchr/testify/require"
)

const rrSetTypeTxt = "TXT"

func TestRrSetRepository_FetchRRSetForZone(t *testing.T) {
	t.Parallel()

	ctx, config, rrSetRepositoryFactory := setupRRSetRepositoryTests(t)

	t.Run("FetchRRSetForZone success", func(t *testing.T) {
		t.Parallel()

		rrSetRepository, err := rrSetRepositoryFactory.NewRRSetRepository(config, "1234")
		require.NoError(t, err)
		rrSet, err := rrSetRepository.FetchRRSetForZone(ctx, "test.com.", rrSetTypeTxt)
		require.NoError(t, err)
		require.Equal(t, *rrSet.Id, "1234")
	})

	t.Run("FetchRRSetForZone failure", func(t *testing.T) {
		t.Parallel()

		rrSetRepository, err := rrSetRepositoryFactory.NewRRSetRepository(config, "5678")
		require.NoError(t, err)
		_, err = rrSetRepository.FetchRRSetForZone(ctx, "test.com.", rrSetTypeTxt)
		require.Error(t, err)
	})

	t.Run("FetchRRSetForZone not found", func(t *testing.T) {
		t.Parallel()

		rrSetRepository, err := rrSetRepositoryFactory.NewRRSetRepository(config, "9999")
		require.NoError(t, err)
		_, err = rrSetRepository.FetchRRSetForZone(ctx, "test.com.", rrSetTypeTxt)
		require.Error(t, err)
		require.ErrorIs(t, err, repository.ErrRRSetNotFound)
	})
}

func TestRrSetRepository_CreateRRSet(t *testing.T) {
	t.Parallel()

	ctx, config, rrSetRepositoryFactory := setupRRSetRepositoryTests(t)

	t.Run("CreateRRSet success", func(t *testing.T) {
		t.Parallel()

		rrSetRepository, err := rrSetRepositoryFactory.NewRRSetRepository(config, "0000")
		require.NoError(t, err)
		err = rrSetRepository.CreateRRSet(ctx, stackitdnsclient.RecordSet{})
		require.NoError(t, err)
	})

	t.Run("CreateRRSet failure", func(t *testing.T) {
		t.Parallel()

		rrSetRepository, err := rrSetRepositoryFactory.NewRRSetRepository(config, "1111")
		require.NoError(t, err)
		err = rrSetRepository.CreateRRSet(ctx, stackitdnsclient.RecordSet{})
		require.Error(t, err)
	})
}

func TestRrSetRepository_UpdateRRSet(t *testing.T) {
	t.Parallel()

	ctx, config, rrSetRepositoryFactory := setupRRSetRepositoryTests(t)

	t.Run("UpdateRRSet success", func(t *testing.T) {
		t.Parallel()

		comment := "test"
		id := "0000"
		name := "test.com."
		ttl := int64(60)
		content := "test"

		rrSetRepository, err := rrSetRepositoryFactory.NewRRSetRepository(config, "2222")
		require.NoError(t, err)
		err = rrSetRepository.UpdateRRSet(
			ctx,
			stackitdnsclient.RecordSet{
				Comment: &comment,
				Id:      &id,
				Name:    &name,
				Ttl:     &ttl,
				Records: &[]stackitdnsclient.Record{{Content: &content}},
			},
		)
		require.NoError(t, err)
	})

	t.Run("UpdateRRSet failure", func(t *testing.T) {
		t.Parallel()

		comment := "test"
		id := "2222"
		name := "test.com."
		ttl := int64(60)
		content := "test"

		rrSetRepository, err := rrSetRepositoryFactory.NewRRSetRepository(config, "3333")
		require.NoError(t, err)
		err = rrSetRepository.UpdateRRSet(
			ctx,
			stackitdnsclient.RecordSet{
				Comment: &comment,
				Id:      &id,
				Name:    &name,
				Ttl:     &ttl,
				Records: &[]stackitdnsclient.Record{{Content: &content}},
			},
		)
		require.Error(t, err)
	})
}

func TestRrSetRepository_DeleteRRSet(t *testing.T) {
	t.Parallel()

	ctx, config, rrSetRepositoryFactory := setupRRSetRepositoryTests(t)

	t.Run("DeleteRRSet success", func(t *testing.T) {
		t.Parallel()

		rrSetRepository, err := rrSetRepositoryFactory.NewRRSetRepository(config, "1234")
		require.NoError(t, err)
		err = rrSetRepository.DeleteRRSet(ctx, "2222")
		require.NoError(t, err)
	})

	t.Run("DeleteRRSet failure", func(t *testing.T) {
		t.Parallel()

		rrSetRepository, err := rrSetRepositoryFactory.NewRRSetRepository(config, "1234")
		require.NoError(t, err)
		err = rrSetRepository.DeleteRRSet(ctx, "3333")
		require.Error(t, err)
	})

	t.Run("DeleteRRSet 400 return", func(t *testing.T) {
		t.Parallel()

		rrSetRepository, err := rrSetRepositoryFactory.NewRRSetRepository(config, "1234")
		require.NoError(t, err)
		err = rrSetRepository.DeleteRRSet(ctx, "4444")
		require.Error(t, err)
		require.ErrorIs(t, err, repository.ErrRRSetNotFound)
	})

	t.Run("DeleteRRSet 404 return", func(t *testing.T) {
		t.Parallel()

		rrSetRepository, err := rrSetRepositoryFactory.NewRRSetRepository(config, "1234")
		require.NoError(t, err)
		err = rrSetRepository.DeleteRRSet(ctx, "5555")
		require.Error(t, err)
		require.ErrorIs(t, err, repository.ErrRRSetNotFound)
	})
}

func setupRRSetRepositoryTests(t *testing.T) (context.Context, repository.Config, repository.RRSetRepositoryFactory) {
	t.Helper()

	ctx := context.TODO()
	server := getTestServer(t)
	t.Cleanup(server.Close)

	config := repository.Config{
		ApiBasePath: server.URL,
		AuthToken:   "test-token",
		ProjectId:   "1234",
		HttpClient:  server.Client(),
	}
	rrSetRepositoryFactory := repository.NewRRSetRepositoryFactory()

	return ctx, config, rrSetRepositoryFactory
}
