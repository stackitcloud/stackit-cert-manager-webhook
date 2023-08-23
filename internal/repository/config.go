package repository

import "net/http"

type Config struct {
	ApiBasePath string
	AuthToken   string
	ProjectId   string
	HttpClient  *http.Client
}
