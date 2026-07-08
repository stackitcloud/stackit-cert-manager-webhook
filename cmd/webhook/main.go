package main

import (
	"net"
	"net/http"
	"os"
	"time"

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

	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   5 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout:   5 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
			IdleConnTimeout:      30 * time.Second,
		},
	}

	cmd.RunWebhookServer(
		GroupName,
		resolver.NewResolver(
			httpClient,
			logger,
			repository.NewZoneRepositoryFactory(),
			repository.NewRRSetRepositoryFactory(),
			resolver.NewSecretFetcher(),
			resolver.NewConfigProvider(),
		),
	)
}
