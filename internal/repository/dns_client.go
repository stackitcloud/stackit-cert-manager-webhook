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
