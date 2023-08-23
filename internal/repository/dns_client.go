package repository

import (
	"fmt"

	stackitdnsclient "github.com/stackitcloud/stackit-dns-api-client-go"
)

func newStackitDnsClient(
	config Config,
) *stackitdnsclient.APIClient {
	configClient := stackitdnsclient.NewConfiguration()
	configClient.DefaultHeader["Authorization"] = fmt.Sprintf("Bearer %s", config.AuthToken)
	configClient.BasePath = config.ApiBasePath
	configClient.HTTPClient = config.HttpClient

	return stackitdnsclient.NewAPIClient(configClient)
}
