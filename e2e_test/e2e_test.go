//go:build e2e
// +build e2e

package e2e_test

import (
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stackitcloud/stackit-cert-manager-webhook/internal/repository"
	"github.com/stackitcloud/stackit-cert-manager-webhook/internal/resolver"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"github.com/cert-manager/cert-manager/test/acme/dns"
)

var (
	zone = os.Getenv("TEST_ZONE_NAME")
	fqdn string
)

func TestRunsSuite(t *testing.T) {
	/* The manifest path should contain a file named config.json that is a
	   snippet of valid configuration that should be included on the
	   ChallengeRequest passed as part of the test cases.*/
	t.Parallel()

	fqdn = getRandomString(20) + "." + zone
	if !strings.HasSuffix(fqdn, ".") {
		fqdn = fmt.Sprintf("%s.", fqdn)
	}

	logger, err := zap.NewProduction()
	assert.NoError(t, err)

	fixture := dns.NewFixture(resolver.NewResolver(
		&http.Client{},
		logger,
		repository.NewZoneRepositoryFactory(),
		repository.NewRRSetRepositoryFactory(),
		resolver.NewSecretFetcher(),
		resolver.NewConfigProvider(),
	),
		dns.SetResolvedZone(zone),
		dns.SetResolvedFQDN(fqdn),
		dns.SetPropagationLimit(15*time.Minute),
		dns.SetPollInterval(20*time.Second),
		dns.SetAllowAmbientCredentials(false),
		dns.SetManifestPath("../testdata/stackit"),
	)

	// RunConformance will execute all conformance tests using the supplied
	// configuration These conformance tests should be run by all external DNS
	// solver webhook implementations, see
	// https://github.com/cert-manager/webhook-example
	fixture.RunConformance(t)
}

func getRandomString(n int) string {
	letters := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}

	return string(b)
}
