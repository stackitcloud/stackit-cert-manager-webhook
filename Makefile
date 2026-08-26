# https://github.com/golangci/golangci-lint/releases
GOLANGCI_VERSION = 2.12.2
HELM_DOCS_VERSION = 1.14.2
LICENCES_IGNORE_LIST = $(shell cat licenses/licenses-ignore-list.txt)

VERSION ?= 0.0.1
IMAGE_TAG_BASE ?= stackitcloud/stackit-cert-manager-webhook
IMG ?= $(IMAGE_TAG_BASE):$(VERSION)

BUILD_VERSION ?= $(shell git branch --show-current)
BUILD_COMMIT ?= $(shell git rev-parse --short HEAD)
BUILD_TIMESTAMP ?= $(shell date -u '+%Y-%m-%d %H:%M:%S')

PWD = $(shell pwd)
export PATH := $(PWD)/bin:$(PATH)

download:
	go mod download

.PHONY: build
build:
	CGO_ENABLED=0 go build -ldflags "-s -w" -o ./bin/stackit-cert-manager-webhook -v cmd/webhook/main.go

.PHONY: docker-build
docker-build:
	docker build -t $(IMG) -f Dockerfile .

test:
	go test -race ./...

mocks:
	go generate ./...

GOLANGCI_LINT = bin/golangci-lint-$(GOLANGCI_VERSION)
$(GOLANGCI_LINT):
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/main/install.sh | bash -s -- -b bin v$(GOLANGCI_VERSION)
	@mv bin/golangci-lint "$(@)"

lint: $(GOLANGCI_LINT) download
	$(GOLANGCI_LINT) run -v

HELM_DOCS = bin/helm-docs
$(HELM_DOCS):
	GOBIN=$(PWD)/bin go install github.com/norwoodj/helm-docs/cmd/helm-docs@v$(HELM_DOCS_VERSION)

helm-docs: $(HELM_DOCS)
	$(HELM_DOCS)

out:
	@mkdir -pv "$(@)"

reports:
	@mkdir -pv "$(@)/licenses"

coverage: out
	go test -race ./... -coverprofile=out/cover.out

html-coverage: out/report.json
	go tool cover -html=out/cover.out

.PHONY: out/report.json
out/report.json:
	go test -race ./... -coverprofile=out/cover.out --json | tee "$(@)"

.PHONY: test-e2e-conformance
test-e2e-conformance:
	@TEST_ZONE_NAME=$(TEST_ZONE_NAME) go test -race -tags=e2e ./... -coverprofile out/cover.out

run:
	go run cmd/webhook/main.go

.PHONY: clean
clean:
	rm -rf ./bin
	rm -rf ./out

GO_RELEASER = bin/goreleaser
$(GO_RELEASER):
	GOBIN=$(PWD)/bin go install github.com/goreleaser/goreleaser@latest

.PHONY: release-check
release-check: $(GO_RELEASER) ## Check if the release will work
	GITHUB_SERVER_URL=github.com GITHUB_REPOSITORY=stackitcloud/stackit-cert-manager-webhook REGISTRY=$(REGISTRY) IMAGE_NAME=$(IMAGE_NAME) $(GO_RELEASER) release --snapshot --clean --skip-publish

GO_LICENSES = bin/go-licenses
$(GO_LICENSES):
	GOBIN=$(PWD)/bin go install github.com/google/go-licenses

.PHONY: license-check
license-check: $(GO_LICENSES) reports ## Check licenses against code.
	$(GO_LICENSES) check --include_tests --ignore $(LICENCES_IGNORE_LIST) ./...

.PHONY: license-report
license-report: $(GO_LICENSES) reports ## Create licenses report against code.
	$(GO_LICENSES) report --include_tests --ignore $(LICENCES_IGNORE_LIST) ./... > ./reports/licenses/licenses-list.csv

# ==============================================================================
# E2E Local Testing
# ==============================================================================

E2E_TMP_DIR = tests/e2e-tmp

.PHONY: build-linux
build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=$(shell go env GOARCH) go build -ldflags "-s -w" -o ./stackit-cert-manager-webhook -v cmd/webhook/main.go

.PHONY: docker-build-e2e
docker-build-e2e: build-linux
	docker build -t stackitcloud/stackit-cert-manager-webhook:e2e -f Dockerfile .
	rm ./stackit-cert-manager-webhook

.PHONY: e2e-check-env
e2e-check-env:
	@if [ -z "$(PROJECT_ID)" ] || [ -z "$(ZONE_NAME)" ] || [ -z "$(AUTH_KEY_PATH)" ]; then \
		echo "Error: Missing PROJECT_ID, ZONE_NAME, or AUTH_KEY_PATH environment variables."; \
		exit 1; \
	fi

.PHONY: e2e-cluster
e2e-cluster: docker-build-e2e
	@echo "=> Creating Kind cluster and loading image..."
	kind create cluster --name stackit-e2e || true
	kind load docker-image stackitcloud/stackit-cert-manager-webhook:e2e --name stackit-e2e

.PHONY: e2e-cert-manager
e2e-cert-manager:
	@echo "=> Installing cert-manager via Helm..."
	helm repo add jetstack https://charts.jetstack.io --force-update
	helm upgrade --install cert-manager jetstack/cert-manager \
		--namespace cert-manager \
		--create-namespace \
		--set crds.enabled=true

	kubectl wait --for=condition=Available --timeout=300s deployment/cert-manager -n cert-manager
	kubectl wait --for=condition=Available --timeout=300s deployment/cert-manager-webhook -n cert-manager

.PHONY: e2e-namespaces
e2e-namespaces:
	@echo "=> Creating test namespaces and secrets..."
	kubectl create namespace e2e-tenant --dry-run=client -o yaml | kubectl apply -f -
	kubectl create secret generic stackit-dynamic-auth -n e2e-tenant \
		--from-file=sa.json=$(AUTH_KEY_PATH) \
		--dry-run=client -o yaml | kubectl apply -f -
	kubectl create namespace e2e-tenant-two --dry-run=client -o yaml | kubectl apply -f -
	kubectl create secret generic stackit-dynamic-auth -n e2e-tenant-two \
		--from-file=sa.json=$(AUTH_KEY_PATH) \
		--dry-run=client -o yaml | kubectl apply -f -

.PHONY: e2e-deploy-webhook
e2e-deploy-webhook:
	@echo "=> Deploying stackit-cert-manager-webhook..."
	kubectl create secret generic stackit-sa-authentication -n cert-manager \
		--from-file=sa.json=$(AUTH_KEY_PATH) \
		--dry-run=client -o yaml | kubectl apply -f -
	helm upgrade --install stackit-cert-manager-webhook ./deploy/stackit \
		--namespace cert-manager \
		--set image.repository=stackitcloud/stackit-cert-manager-webhook \
		--set image.tag=e2e \
		--set image.pullPolicy=Never \
		--set stackitSaAuthentication.enabled=true \
		--set stackitSaAuthentication.secretAccessScope=issuer
	kubectl wait --for=condition=available --timeout=120s deployment/stackit-cert-manager-webhook -n cert-manager

.PHONY: e2e-run-kuttl
e2e-run-kuttl:
	@echo "=> Preparing test manifests..."
	rm -rf $(E2E_TMP_DIR)
	cp -r tests/e2e $(E2E_TMP_DIR)
	find $(E2E_TMP_DIR) -type f -name "*.yaml" -exec sed -i.bak "s/\$${PROJECT_ID}/$(PROJECT_ID)/g" {} +
	find $(E2E_TMP_DIR) -type f -name "*.yaml" -exec sed -i.bak "s/\$${ZONE_NAME}/$(ZONE_NAME)/g" {} +
	find $(E2E_TMP_DIR) -type f -name "*.bak" -delete

	@echo "=> Running Kuttl Tests..."
	cd $(E2E_TMP_DIR) && kubectl kuttl test

.PHONY: clean-e2e-local
clean-e2e-local:
	@echo "=> Cleaning up local test environment..."
	kind delete cluster --name stackit-e2e
	rm -rf $(E2E_TMP_DIR)

# The main target chains the dependencies together
.PHONY: test-e2e-local
test-e2e-local: e2e-check-env e2e-cluster e2e-cert-manager e2e-namespaces e2e-deploy-webhook
	@$(MAKE) e2e-run-kuttl || ( $(MAKE) clean-e2e-local && exit 1 )
	@$(MAKE) clean-e2e-local