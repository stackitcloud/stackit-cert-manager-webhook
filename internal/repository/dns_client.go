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
	return newStackitDnsClient(
		stackitconfig.WithToken(config.AuthToken),
		stackitconfig.WithHTTPClient(config.HttpClient),
		stackitconfig.WithEndpoint(config.ApiBasePath),
	)
}

func newStackitDnsClientKeyPath(config Config) (*stackitdnsclient.APIClient, error) {
	return newStackitDnsClient(
		stackitconfig.WithServiceAccountKeyPath(config.SaKeyPath),
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
