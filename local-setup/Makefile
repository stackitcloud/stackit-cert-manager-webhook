# variables
CLUSTER_NAME := stackit-webhook-dev
KIND_KUBECONFIG := $(HOME)/.kube/config
CERT_MANAGER_VERSION := v1.13.3

.PHONY: cluster-up cluster-down install-cert-manager status

## -- Iteration 1: Foundation --

cluster-up:
	@echo "spinning up kind cluster..."
	kind create cluster --name $(CLUSTER_NAME)

cluster-down:
	@echo "destroying kind cluster..."
	kind delete cluster --name $(CLUSTER_NAME)

install-cert-manager:
	@echo "installing cert-manager..."
	kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/$(CERT_MANAGER_VERSION)/cert-manager.yaml
	@echo "waiting for cert-manager pods to be ready..."
	kubectl wait --namespace cert-manager \
		--for=condition=ready pod \
		--all \
		--timeout=90s

## -- The "Helm Hijack" Local Dev Setup --

HELM_CHART_PATH ?= ../deploy/stackit
RELEASE_NAME ?= stackit-webhook
NAMESPACE ?= cert-manager

setup-local-helm:
	@echo "1. installing the official helm chart (replicas=0)..."
	helm upgrade --install $(RELEASE_NAME) $(HELM_CHART_PATH) \
		--namespace $(NAMESPACE) \
		--set replicaCount=0

	@echo "2. dynamically discovering and patching the Service..."
	@SVC_NAME=$$(kubectl get svc -n $(NAMESPACE) -l release=$(RELEASE_NAME) -o jsonpath='{.items[0].metadata.name}'); \
	echo "   -> found service: $$SVC_NAME"; \
	kubectl patch svc $$SVC_NAME -n $(NAMESPACE) --type='json' -p='[{"op": "remove", "path": "/spec/selector"}]'

	@echo "3. bridging the Service to your laptop..."
	@HOST_IP=$$(docker exec $(CLUSTER_NAME)-control-plane getent hosts host.docker.internal | awk '{ print $$1 }'); \
	SVC_NAME=$$(kubectl get svc -n $(NAMESPACE) -l release=$(RELEASE_NAME) -o jsonpath='{.items[0].metadata.name}'); \
	sed -e "s/RELEASE_NAME_PLACEHOLDER/$$SVC_NAME/g" \
	    -e "s/NAMESPACE_PLACEHOLDER/$(NAMESPACE)/g" \
	    -e "s/HOST_IP_PLACEHOLDER/$$HOST_IP/g" \
	    webhook-endpoints.yaml.tmpl | kubectl apply -f -

	@echo "4. generating a strictly valid TLS Certificate for the bridge..."
	@SVC_NAME=$$(kubectl get svc -n $(NAMESPACE) -l release=$(RELEASE_NAME) -o jsonpath='{.items[0].metadata.name}'); \
	sed -e "s/SVC_NAME_PLACEHOLDER/$$SVC_NAME/g" \
	    -e "s/NAMESPACE_PLACEHOLDER/$(NAMESPACE)/g" \
	    webhook-local-cert.yaml.tmpl | kubectl apply -f -

	@echo "5. waiting for certificate generation and patching APIService..."
	@kubectl wait --namespace $(NAMESPACE) --for=condition=ready certificate local-dev-webhook-cert --timeout=60s
	@kubectl annotate apiservice v1alpha1.acme.stackit.de cert-manager.io/inject-ca-from=$(NAMESPACE)/local-dev-webhook-cert --overwrite

	@echo "6. extracting the perfect certificates to ./certs/..."
	@mkdir -p certs
	@kubectl get secret local-dev-webhook-tls -n $(NAMESPACE) --template='{{ index .data "tls.crt" | base64decode }}' > certs/tls.crt
	@kubectl get secret local-dev-webhook-tls -n $(NAMESPACE) --template='{{ index .data "tls.key" | base64decode }}' > certs/tls.key

	@echo "✅ helm hijack complete! you can now run 'make run-webhook'."

## -- Iteration 4: Trigger the Webhook --

API_BASE_PATH ?= https://dns.api.stackit.cloud

test-webhook:
	@if [ -z "$(PROJECT_ID)" ]; then echo "❌ error: PROJECT_ID is required."; exit 1; fi
	@if [ -z "$(AUTH_TOKEN)" ]; then echo "❌ error: AUTH_TOKEN is required."; exit 1; fi
	@if [ -z "$(DOMAIN)" ]; then echo "❌ error: DOMAIN is required."; exit 1; fi
	@if [ -z "$(EMAIL)" ]; then echo "❌ error: EMAIL is required."; exit 1; fi
	@echo "deploying test resources to the cluster (targeting API: $(API_BASE_PATH))..."
	@sed -e 's/PROJECT_ID_PLACEHOLDER/$(PROJECT_ID)/g' \
	     -e 's/AUTH_TOKEN_PLACEHOLDER/$(AUTH_TOKEN)/g' \
	     -e 's/DOMAIN_PLACEHOLDER/$(DOMAIN)/g' \
	     -e 's|API_BASE_PATH_PLACEHOLDER|$(API_BASE_PATH)|g' \
	     -e 's|EMAIL_PLACEHOLDER|$(EMAIL)|g' \
	     test-resources.yaml.tmpl | kubectl apply -f -
	@echo "✅ test resources applied! watch your webhook logs."

test-status:
	@echo "checking challenge status..."
	kubectl get challenges,orders,certificates -n default

status:
	@echo "checking cluster and cert-manager status..."
	kubectl get pods -n cert-manager