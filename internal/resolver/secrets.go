package resolver

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

//go:generate mockgen -destination=./mock/secrets.go -source=./secrets.go SecretFetcher
type SecretFetcher interface {
	StringFromSecret(namespace, secretName, key string) (string, error)
}

type kubeSecretFetcher struct {
	client kubernetes.Interface
	ctx    context.Context
}

func (k *kubeSecretFetcher) StringFromSecret(namespace, secretName, key string) (string, error) {
	secret, err := k.client.CoreV1().Secrets(namespace).Get(k.ctx, secretName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	binary, ok := secret.Data[key]
	if !ok {
		return "", fmt.Errorf("key `%q` not found in secretFetcher `%s/%s`",
			key, namespace, secretName)
	}

	return string(binary), nil
}

func NewSecretFetcher() SecretFetcher {
	return &kubeSecretFetcher{}
}
