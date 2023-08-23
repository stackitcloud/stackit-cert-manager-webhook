package repository_test

import (
	"context"
	"testing"

	"github.com/stackitcloud/stackit-cert-manager-webhook/internal/repository"
	stackitdnsclient "github.com/stackitcloud/stackit-dns-api-client-go"
	"github.com/stretchr/testify/assert"
)

const rrSetTypeTxt = "TXT"

func TestRrSetRepository_FetchRRSetForZone(t *testing.T) {
	t.Parallel()

	ctx, config, rrSetRepositoryFactory := setupRRSetRepositoryTests(t)

	t.Run("FetchRRSetForZone success", func(t *testing.T) {
		t.Parallel()

		rrSetRepository := rrSetRepositoryFactory.NewRRSetRepository(config, "1234")
		rrSet, err := rrSetRepository.FetchRRSetForZone(ctx, "test.com.", rrSetTypeTxt)
		assert.NoError(t, err)
		assert.Equal(t, rrSet.Id, "1234")
	})

	t.Run("FetchRRSetForZone failure", func(t *testing.T) {
		t.Parallel()

		rrSetRepository := rrSetRepositoryFactory.NewRRSetRepository(config, "5678")
		_, err := rrSetRepository.FetchRRSetForZone(ctx, "test.com.", rrSetTypeTxt)
		assert.Error(t, err)
	})

	t.Run("FetchRRSetForZone not found", func(t *testing.T) {
		t.Parallel()

		rrSetRepository := rrSetRepositoryFactory.NewRRSetRepository(config, "9999")
		_, err := rrSetRepository.FetchRRSetForZone(ctx, "test.com.", rrSetTypeTxt)
		assert.Error(t, err)
		assert.ErrorIs(t, err, repository.ErrRRSetNotFound)
	})
}

func TestRrSetRepository_CreateRRSet(t *testing.T) {
	t.Parallel()

	ctx, config, rrSetRepositoryFactory := setupRRSetRepositoryTests(t)

	t.Run("CreateRRSet success", func(t *testing.T) {
		t.Parallel()

		rrSetRepository := rrSetRepositoryFactory.NewRRSetRepository(config, "0000")
		err := rrSetRepository.CreateRRSet(ctx, stackitdnsclient.RrsetRrSetPost{})
		assert.NoError(t, err)
	})

	t.Run("CreateRRSet failure", func(t *testing.T) {
		t.Parallel()

		rrSetRepository := rrSetRepositoryFactory.NewRRSetRepository(config, "1111")
		err := rrSetRepository.CreateRRSet(ctx, stackitdnsclient.RrsetRrSetPost{})
		assert.Error(t, err)
	})
}

func TestRrSetRepository_UpdateRRSet(t *testing.T) {
	t.Parallel()

	ctx, config, rrSetRepositoryFactory := setupRRSetRepositoryTests(t)

	t.Run("UpdateRRSet success", func(t *testing.T) {
		t.Parallel()

		rrSetRepository := rrSetRepositoryFactory.NewRRSetRepository(config, "2222")
		err := rrSetRepository.UpdateRRSet(
			ctx,
			stackitdnsclient.DomainRrSet{
				Comment: "test",
				Id:      "0000",
				Name:    "test.com.",
				Ttl:     60,
				Records: []stackitdnsclient.DomainRecord{{Content: "test"}},
			},
		)
		assert.NoError(t, err)
	})

	t.Run("UpdateRRSet failure", func(t *testing.T) {
		t.Parallel()

		rrSetRepository := rrSetRepositoryFactory.NewRRSetRepository(config, "3333")
		err := rrSetRepository.UpdateRRSet(
			ctx,
			stackitdnsclient.DomainRrSet{
				Comment: "test",
				Id:      "2222",
				Name:    "test.com.",
				Ttl:     60,
				Records: []stackitdnsclient.DomainRecord{{Content: "test"}},
			},
		)
		assert.Error(t, err)
	})
}

func TestRrSetRepository_DeleteRRSet(t *testing.T) {
	t.Parallel()

	ctx, config, rrSetRepositoryFactory := setupRRSetRepositoryTests(t)

	t.Run("DeleteRRSet success", func(t *testing.T) {
		t.Parallel()

		rrSetRepository := rrSetRepositoryFactory.NewRRSetRepository(config, "1234")
		err := rrSetRepository.DeleteRRSet(ctx, "2222")
		assert.NoError(t, err)
	})

	t.Run("DeleteRRSet failure", func(t *testing.T) {
		t.Parallel()

		rrSetRepository := rrSetRepositoryFactory.NewRRSetRepository(config, "1234")
		err := rrSetRepository.DeleteRRSet(ctx, "3333")
		assert.Error(t, err)
	})

	t.Run("DeleteRRSet 400 return", func(t *testing.T) {
		t.Parallel()

		rrSetRepository := rrSetRepositoryFactory.NewRRSetRepository(config, "1234")
		err := rrSetRepository.DeleteRRSet(ctx, "4444")
		assert.Error(t, err)
		assert.ErrorIs(t, err, repository.ErrRRSetNotFound)
	})

	t.Run("DeleteRRSet 404 return", func(t *testing.T) {
		t.Parallel()

		rrSetRepository := rrSetRepositoryFactory.NewRRSetRepository(config, "1234")
		err := rrSetRepository.DeleteRRSet(ctx, "5555")
		assert.Error(t, err)
		assert.ErrorIs(t, err, repository.ErrRRSetNotFound)
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
