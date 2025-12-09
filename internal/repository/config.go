package repository

import "net/http"

type Config struct {
	ApiBasePath           string
	ServiceAccountBaseUrl string
	AuthToken             string
	ProjectId             string
	HttpClient            *http.Client
	SaKeyPath             string
	UseSaKey              bool
}
