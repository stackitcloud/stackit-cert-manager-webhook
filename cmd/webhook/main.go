package main

import (
	"net/http"
	"os"

	"github.com/cert-manager/cert-manager/pkg/acme/webhook/cmd"
	"github.com/stackitcloud/stackit-cert-manager-webhook/internal/repository"
	"github.com/stackitcloud/stackit-cert-manager-webhook/internal/resolver"
	"go.uber.org/zap"
)

// GroupName is the K8s API group.
var GroupName = os.Getenv("GROUP_NAME")

func main() {
	if GroupName == "" {
		panic("GROUP_NAME must be specified")
	}

	logger, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}

	// This will register our custom DNS provider with the webhook serving
	// library, making it available as an API under the provided GroupName.
	// You can register multiple DNS provider implementations with a single
	// webhook, where the Name() method will be used to disambiguate between
	// the different implementations.
	cmd.RunWebhookServer(
		GroupName,
		resolver.NewResolver(
			&http.Client{},
			logger,
			repository.NewZoneRepositoryFactory(),
			repository.NewRRSetRepositoryFactory(),
			resolver.NewSecretFetcher(),
			resolver.NewConfigProvider(),
		),
	)
}
