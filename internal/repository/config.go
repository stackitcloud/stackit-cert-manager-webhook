package repository

import stackitdnsclient "github.com/stackitcloud/stackit-sdk-go/services/dns/v1api"

type Config struct {
	ProjectId string
	ApiClient *stackitdnsclient.APIClient
}
