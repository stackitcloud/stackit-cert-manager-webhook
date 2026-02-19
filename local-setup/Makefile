CLUSTER_NAME := stackit-webhook-dev
KUBECONFIG ?= $(HOME)/.kube/config # supposed to be the kind kubeconfig
GROUP_NAME := acme.stackit.de
STAMPS_DIR := .make_stamps
CERTS_DIR := certs

## -- Foundation --

CERT_MANAGER_VERSION := v1.13.3

$(STAMPS_DIR):
	@mkdir -p "$(STAMPS_DIR)"

$(STAMPS_DIR)/.cluster-up: | $(STAMPS_DIR)
	@echo "spinning up kind cluster..."
	kind create cluster --name $(CLUSTER_NAME)
	@touch $@

.PHONY: cluster-down
cluster-down:
	@echo "destroying kind cluster..."
	kind delete cluster --name $(CLUSTER_NAME)
	@rm -rf $(STAMPS_DIR)
	@rm -rf $(CERTS_DIR)

$(STAMPS_DIR)/.cert-manager-ready: $(STAMPS_DIR)/.cluster-up
	@echo "installing cert-manager..."
	kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/$(CERT_MANAGER_VERSION)/cert-manager.yaml
	@echo "waiting for cert-manager pods to be ready..."
	kubectl wait --namespace cert-manager \
		--for=condition=Available deployment \
		--all \
		--timeout=90s
	@touch $@

## -- "Helm Hijack" Local Dev Setup --

HELM_CHART_PATH ?= ../deploy/stackit
RELEASE_NAME ?= stackit-webhook
NAMESPACE ?= cert-manager


$(STAMPS_DIR)/.helm-setup-ready: $(STAMPS_DIR)/.cert-manager-ready
	@echo "0. cleaning up previously hacked resources to avoid Helm conflicts..."
	@kubectl delete apiservice v1alpha1.acme.stackit.de --ignore-not-found
	@kubectl delete svc -n $(NAMESPACE) -l release=$(RELEASE_NAME) --ignore-not-found

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
	@kubectl annotate apiservice v1alpha1.$(GROUP_NAME) cert-manager.io/inject-ca-from=$(NAMESPACE)/local-dev-webhook-cert --overwrite
	@echo "✅ helm setup complete!"
	@touch $@

$(CERTS_DIR)/tls.crt $(CERTS_DIR)/tls.key: $(STAMPS_DIR)/.helm-setup-ready
	@echo "extracting the certificates to ./certs/..."
	@mkdir -p $(CERTS_DIR)
	@kubectl get secret local-dev-webhook-tls -n $(NAMESPACE) --template='{{ index .data "tls.crt" | base64decode }}' > $(CERTS_DIR)/tls.crt
	@kubectl get secret local-dev-webhook-tls -n $(NAMESPACE) --template='{{ index .data "tls.key" | base64decode }}' > $(CERTS_DIR)/tls.key


## -- Run the Webhook Locally --

# note: if you run the makefile from a subdirectory but execute from root, adjust this to local-setup/certs
WEBHOOK_ENTRYPOINT := ../cmd/webhook/main.go

.PHONY: run-webhook
run-webhook: $(CERTS_DIR)/tls.crt $(CERTS_DIR)/tls.key
	@echo "🚀 starting stackit webhook locally on port 8443..."
	GROUP_NAME=$(GROUP_NAME) go run $(WEBHOOK_ENTRYPOINT) \
		--tls-cert-file=$(CERTS_DIR)/tls.crt \
		--tls-private-key-file=$(CERTS_DIR)/tls.key \
		--secure-port=8443 \
		--kubeconfig=$(KUBECONFIG) \
		--authentication-kubeconfig=$(KUBECONFIG) \
		--authorization-kubeconfig=$(KUBECONFIG)


## -- Trigger the Webhook --

API_BASE_PATH ?= https://dns.api.stackit.cloud

.PHONY: test-webhook
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
