package resolver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestStringFromSecret(t *testing.T) {
	t.Parallel()

	client := fake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "test-namespace",
		},
		Data: map[string][]byte{
			"test-key": []byte("test-value"),
		},
	})

	fetcher := &kubeSecretFetcher{
		client: client,
		ctx:    context.TODO(),
	}

	// check for expected value
	value, err := fetcher.StringFromSecret("test-namespace", "test-secret", "test-key")
	assert.NoError(t, err)
	assert.Equal(t, "test-value", value)

	// check for a non-existent key
	_, err = fetcher.StringFromSecret("test-namespace", "test-secret", "non-existent-key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "key `\"non-existent-key\"` not found in secretFetcher `test-namespace/test-secret`")

	// check for a non-existent secret
	_, err = fetcher.StringFromSecret("test-namespace", "non-existent-secret", "test-key")
	assert.Error(t, err)
}
