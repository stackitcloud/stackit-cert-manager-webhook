package repository

import (
	stackitconfig "github.com/stackitcloud/stackit-sdk-go/core/config"
	stackitdnsclient "github.com/stackitcloud/stackit-sdk-go/services/dns"
)

func newStackitDnsClient(
	stackitConfig ...stackitconfig.ConfigurationOption,
) (*stackitdnsclient.APIClient, error) {
	return stackitdnsclient.NewAPIClient(stackitConfig...)
}

func newStackitDnsClientBearerToken(config Config) (*stackitdnsclient.APIClient, error) {
	httpClient := *config.HttpClient

	return newStackitDnsClient(
		stackitconfig.WithToken(config.AuthToken),
		stackitconfig.WithHTTPClient(&httpClient),
		stackitconfig.WithEndpoint(config.ApiBasePath),
	)
}

func newStackitDnsClientKeyPath(config Config) (*stackitdnsclient.APIClient, error) {
	httpClient := *config.HttpClient

	return newStackitDnsClient(
		stackitconfig.WithServiceAccountKeyPath(config.SaKeyPath),
		stackitconfig.WithHTTPClient(&httpClient),
		stackitconfig.WithEndpoint(config.ApiBasePath),
		stackitconfig.WithTokenEndpoint(config.ServiceAccountBaseUrl),
	)
}

func chooseNewStackitDnsClient(config Config) (*stackitdnsclient.APIClient, error) {
	switch {
	case config.UseSaKey:
		return newStackitDnsClientKeyPath(config)
	default:
		return newStackitDnsClientBearerToken(config)
	}
}
