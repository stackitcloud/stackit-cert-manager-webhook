# STACKIT Webhook Integration for Cert Manager

[![GoTemplate](https://img.shields.io/badge/go/template-black?logo=go)](https://github.com/golang-standards/project-layout)
[![Go](https://img.shields.io/badge/go-1.22.0-blue?logo=go)](https://golang.org/)
[![Helm](https://img.shields.io/badge/helm-3.12.3-blue?logo=helm)](https://helm.sh/)
[![Kubernetes](https://img.shields.io/badge/kubernetes-1.30.2-blue?logo=kubernetes)](https://kubernetes.io/)
[![Cert Manager](https://img.shields.io/badge/cert--manager-1.15.2-blue?logo=cert-manager)](https://cert-manager.io/)
[![Releases](https://img.shields.io/github/v/release/stackitcloud/stackit-cert-manager-webhook?include_prereleases)](https://github.com/stackitcloud/stackit-cert-manager-webhook/releases)
[![CI](https://github.com/stackitcloud/stackit-cert-manager-webhook/actions/workflows/main.yml/badge.svg)](https://github.com/stackitcloud/stackit-cert-manager-webhook/actions/workflows/main.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/stackitcloud/stackit-cert-manager-webhook)](https://goreportcard.com/report/github.com/stackitcloud/stackit-cert-manager-webhook)

Facilitate a webhook integration for leveraging the STACKIT DNS alongside its [API](https://docs.api.stackit.cloud/documentation/dns/version/v1) to act as a DNS01 ACME Issuer with [cert-manager](https://cert-manager.io/docs/).

## Installation

```bash
helm repo add stackit-cert-manager-webhook [https://stackitcloud.github.io/stackit-cert-manager-webhook](https://stackitcloud.github.io/stackit-cert-manager-webhook)
helm install stackit-cert-manager-webhook --namespace cert-manager stackit-cert-manager-webhook/stackit-cert-manager-webhook
```

## Authentication & Usage

The STACKIT webhook requires authentication against the STACKIT DNS API. Depending on your cluster architecture and security policies, you can authenticate using one of the three methods below.

The webhook will explicitly fail if multiple mutually exclusive authentication methods are configured for a single Issuer.

### Option A: Dynamic Service Account Key (Multi-Tenant)

This method is recommended for multi-tenant clusters where different `Issuer` or `ClusterIssuer` resources manage zones across different STACKIT projects. The webhook fetches the Service Account JSON directly from a Kubernetes Secret per challenge.

1. **Create the Secret containing the SA JSON:**
   ```bash
   kubectl create secret generic stackit-tenant-a-auth \
     -n default \
     --from-file=sa.json=/path/to/tenant-a-sa.json
   ```

2. **Configure the Issuer:**
   Ensure the `serviceAccountSecretNamespace` matches the namespace of your Secret. If you want the webhook to read secrets outside of its own installation namespace, you must set `stackitSaAuthentication.secretAccessScope=issuer` when installing the Helm chart.
   ```yaml
   apiVersion: cert-manager.io/v1
   kind: Issuer
   metadata:
     name: letsencrypt-prod
     namespace: default
   spec:
     acme:
       server: [https://acme-v02.api.letsencrypt.org/directory](https://acme-v02.api.letsencrypt.org/directory)
       email: example@example.com
       privateKeySecretRef:
         name: letsencrypt-prod
       solvers:
       - dns01:
           webhook:
             solverName: stackit
             groupName: acme.stackit.de
             config:
               projectId: <STACKIT ID PROJECT>
               serviceAccountSecretRef: stackit-tenant-a-auth
               serviceAccountSecretKey: sa.json
               serviceAccountSecretNamespace: default
   ```

### Option B: Static Service Account Key (Single Tenant / Global Fallback)

This method mounts a single Service Account key JSON file into the webhook Pod. It is ideal for single-tenant clusters where the webhook manages domains for a single STACKIT project or organization.

1. **Deploy the Webhook with the Key Mounted:**
   Create a secret in the `cert-manager` namespace and install the Helm chart with mounting enabled:
   ```bash
   kubectl create secret generic stackit-sa-authentication \
     -n cert-manager \
     --from-file=sa.json=/path/to/global-sa.json

   helm upgrade --install stackit-cert-manager-webhook stackit-cert-manager-webhook/stackit-cert-manager-webhook \
     --namespace cert-manager \
     --set stackitSaAuthentication.enabled=true
   ```

2. **Configure the Issuer:**
   Reference the mounted file path.
   ```yaml
   apiVersion: cert-manager.io/v1
   kind: ClusterIssuer
   metadata:
     name: letsencrypt-prod
   spec:
     acme:
       server: [https://acme-v02.api.letsencrypt.org/directory](https://acme-v02.api.letsencrypt.org/directory)
       email: example@example.com
       privateKeySecretRef:
         name: letsencrypt-prod
       solvers:
       - dns01:
           webhook:
             solverName: stackit
             groupName: acme.stackit.de
             config:
               projectId: <STACKIT ID PROJECT>
               serviceAccountKeyPath: /var/run/secrets/stackit/sa.json
   ```

### Option C: Workload Identity Federation (WIF)

If your cluster supports Workload Identity Federation (e.g., SKE clusters), you can avoid managing long-lived keys entirely by projecting a short-lived token into the webhook container.

1. **Annotate the Webhook ServiceAccount:**
   Update your Helm deployment to instruct the identity webhook to inject the federated token.
   ```yaml
   # values.yaml
   serviceAccount:
     annotations:
       workload-identity.stackit.cloud/service-account-email: "your-service-account@sa.stackit.cloud"
   ```

2. **Configure the Issuer:**
   Explicitly instruct the webhook to use the WIF flow.
   ```yaml
   apiVersion: cert-manager.io/v1
   kind: ClusterIssuer
   metadata:
     name: letsencrypt-prod
   spec:
     acme:
       # ...
       solvers:
       - dns01:
           webhook:
             solverName: stackit
             groupName: acme.stackit.de
             config:
               projectId: <STACKIT ID PROJECT>
               useWorkloadIdentityFederation: true
   ```

## Config Options

The following table delineates the configuration options available under the `config` block of the STACKIT Cert Manager Webhook solver:

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `projectId` | string | `""` | **Required.** The unique identifier for the STACKIT project. |
| `apiBasePath` | string | `"https://dns.api.stackit.cloud"` | The base path for the STACKIT DNS API. |
| `serviceAccountSecretRef` | string | `""` | Name of the Kubernetes Secret containing the SA JSON. |
| `serviceAccountSecretKey` | string | `""` | The key within the Secret mapped to the JSON content. |
| `serviceAccountSecretNamespace` | string | `<webhook-namespace>` | The namespace where the Secret is located. |
| `serviceAccountKeyPath` | string | `""` | The absolute file path to a statically mounted SA JSON key inside the webhook container. |
| `useWorkloadIdentityFederation` | bool | `false` | Explicitly enables STACKIT Workload Identity Federation authentication. |
| `serviceAccountBaseUrl` | string | `""` | Custom URL for trading SA keys for access tokens. |
| `acmeTxtRecordTTL` | int32 | `600` | The TTL for the ACME TXT challenge record. |

## Test Procedures

### Unit Testing:
```bash
make test
```

### Unit Testing with Coverage Analysis:
```bash
make coverage
```

### Linting:
```bash
make lint
```

### Go Conformance Testing:
Runs the official cert-manager Go solver test suite in memory against the STACKIT API:
```bash
AUTH_KEY_PATH="<path-to-sa-key.json>" TEST_ZONE_NAME="example.com" make test-e2e-conformance
```
Follow the comprehensive guide available [here](e2e_test/README.md).

### Kubernetes Integration (E2E) Testing:
Spins up a local Kind cluster, installs cert-manager, builds and deploys the webhook, and executes Kuttl integration tests covering both single-tenant (static fallback) and multi-tenant (dynamic SA fetching) flows against Let's Encrypt Staging:

```bash
make test-e2e-local \
  PROJECT_ID="<your-project-id>" \
  ZONE_NAME="<your-test-zone>" \
  AUTH_KEY_PATH="<path-to-sa-key.json>"
```
Follow the comprehensive guide available [here](tests/e2e/README.md).

## Release Process Overview

Our release pipeline leverages goreleaser for the generation and publishing of release assets.
This sophisticated approach ensures the streamlined delivery of:

- Pre-compiled binaries tailored for various platforms.
- Docker images optimized for production readiness.

However, one should be cognizant of the fact that goreleaser doesn't inherently support Helm chart distributions
as part of its conventional workflow. Historically, the incorporation of Helm charts into our releases demanded manual
intervention. Post the foundational release generation via goreleaser, the Helm chart was affixed as an asset through
manual processes.    
For those interested in the Helm chart creation mechanics, the process was facilitated via the command:

```bash
helm package deploy/stackit
```

To release a new version of the Helm chart, one must meticulously update the appVersion and (chart)version delineation in the
[Chart.yaml](./deploy/stackit/Chart.yaml). Post this modification, initiate a new release to encompass these changes.
