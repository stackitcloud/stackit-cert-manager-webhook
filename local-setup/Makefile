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

## -- Iteration 2: Certificates (The Native Way) --

gen-certs:
	@echo "asking cert-manager to generate certificates..."
	kubectl apply -f webhook-certs.yaml
	@echo "waiting for certificate to be ready..."
	kubectl wait --namespace default \
		--for=condition=ready certificate \
		stackit-webhook-cert \
		--timeout=30s
	@echo "extracting certificates to ./certs/..."
	@mkdir -p certs
	kubectl get secret stackit-webhook-tls -n default \
		--template='{{ index .data "tls.crt" | base64decode }}' > certs/tls.crt
	kubectl get secret stackit-webhook-tls -n default \
		--template='{{ index .data "tls.key" | base64decode }}' > certs/tls.key
	@echo "✅ certificates successfully extracted to ./certs/"

## -- Iteration 3: The Network Bridge --

bridge-up:
	@echo "detecting host IP via host.docker.internal..."
	$(eval HOST_IP=$(shell docker exec $(CLUSTER_NAME)-control-plane getent hosts host.docker.internal | awk '{ print $$1 }'))
	@echo "host IP detected as $(HOST_IP)"
	@echo "creating Service and Endpoints to route traffic to $(HOST_IP):8443..."
	@sed 's/HOST_IP_PLACEHOLDER/$(HOST_IP)/g' webhook-bridge.yaml.tmpl | kubectl apply -f -
	@echo "✅ network bridge established."

bridge-status:
	@kubectl get endpoints stackit-webhook -n default

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