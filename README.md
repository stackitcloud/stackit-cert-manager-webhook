# STACKIT Webhook Integration for Cert Manager

[![GoTemplate](https://img.shields.io/badge/go/template-black?logo=go)](https://github.com/golang-standards/project-layout)
[![Go](https://img.shields.io/badge/go-1.27.3-blue?logo=go)](https://golang.org/)
[![Helm](https://img.shields.io/badge/helm-3.x-blue?logo=helm)](https://helm.sh/)
[![Kubernetes](https://img.shields.io/badge/kubernetes-1.37.2-blue?logo=kubernetes)](https://kubernetes.io/)
[![Cert Manager](https://img.shields.io/badge/cert--manager-1.21.3-blue?logo=cert-manager)](https://cert-manager.io/)
[![Releases](https://img.shields.io/github/v/release/stackitcloud/stackit-cert-manager-webhook?include_prereleases)](https://github.com/stackitcloud/stackit-cert-manager-webhook/releases)
[![CI](https://github.com/stackitcloud/stackit-cert-manager-webhook/actions/workflows/main.yml/badge.svg)](https://github.com/stackitcloud/stackit-cert-manager-webhook/actions/workflows/main.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/stackitcloud/stackit-cert-manager-webhook)](https://goreportcard.com/report/github.com/stackitcloud/stackit-cert-manager-webhook)

Facilitate a webhook integration for leveraging the STACKIT DNS alongside its [API](https://docs.api.stackit.cloud/documentation/dns/version/v1) to act as a DNS01 ACME Issuer with [cert-manager](https://cert-manager.io/docs/).

> **Upgrading from v0.x?** `v1.0.0` introduces breaking security and authentication changes (including namespace-scoped Secret RBAC by default and removal of legacy bearer token authentication). See [Migrating from v0.x to v1.0.0](#migrating-from-v0x-to-v100) below.

## Installation

```bash
helm repo add stackit-cert-manager-webhook https://stackitcloud.github.io/stackit-cert-manager-webhook
helm install stackit-cert-manager-webhook --namespace cert-manager stackit-cert-manager-webhook/stackit-cert-manager-webhook
```

## Secret Access Scope & RBAC Modes

By default, the Helm chart does **not** grant cluster-wide Kubernetes `Secret` read permissions. Use `stackitSaAuthentication.secretAccessScope` to match your cluster model:

| `secretAccessScope` | Kubernetes RBAC Created | Allowed Dynamic Secret Lookup (`serviceAccountSecretRef`) | Recommended Use Case |
|---|---|---|---|
| `webhook` *(default)* | Namespace `Role` (`get` on `secrets` in webhook namespace) | Only `ClusterIssuer`s or `Issuer`s in the webhook namespace (`cert-manager`) | Single-tenant clusters using a `ClusterIssuer` with a Secret in `cert-manager` |
| `issuer` | `ClusterRole` (`get` on `secrets`) | `Issuer`s may **only** read Secrets in their own namespace (`serviceAccountSecretNamespace` must match the `Issuer` namespace) | Multi-tenant clusters where each namespace manages its own STACKIT project Secret |
| `none` | None (no `secret-reader` Role/ClusterRole) | Disabled (any `serviceAccountSecretRef` is rejected) | WIF, `DefaultAuth` environment variables, or mounted static Service Account key |

*(Note: When using `secretAccessScope=webhook` with a `ClusterIssuer`, the webhook must be installed in the same namespace as `cert-manager`'s `--cluster-resource-namespace`, which defaults to `cert-manager`.)*

## Authentication & Usage

The STACKIT webhook requires authentication against the STACKIT DNS API. Depending on your cluster architecture and security policies, configure one of the methods below.

The webhook will explicitly fail if multiple mutually exclusive solver authentication fields (`serviceAccountSecretRef`, `serviceAccountKeyPath`, `useWorkloadIdentityFederation`) are set on the same `Issuer`. When none of those fields are specified, the webhook uses the STACKIT SDK's automatic **`DefaultAuth`** chain.

> **Multi-Tenant Security Note:** Pod-level credentials (`DefaultAuth` environment variables, `stackitSaAuthentication.enabled=true`, or pod-level Workload Identity Federation) are shared by the webhook Pod and can be used by any `Issuer` referencing the solver. In untrusted multi-tenant clusters, keep `stackitSaAuthentication.enabled=false`, do not attach ambient credentials to the webhook Pod, and use **Option A** with `stackitSaAuthentication.secretAccessScope=issuer`.

### Option A: Dynamic Service Account Key (Multi-Tenant or `ClusterIssuer`)

This method fetches the Service Account JSON directly from a Kubernetes `Secret` per challenge. Cross-namespace Secret access is strictly blocked: an `Issuer` can only reference a `Secret` in its own namespace.

1. **Install the Webhook with `issuer` Scope (for multi-namespace `Issuer`s):**
   *(Skip `--set stackitSaAuthentication.secretAccessScope=issuer` if you only use a `ClusterIssuer` with Secrets in `cert-manager`.)*
   ```bash
   helm upgrade --install stackit-cert-manager-webhook stackit-cert-manager-webhook/stackit-cert-manager-webhook \
     --namespace cert-manager \
     --set stackitSaAuthentication.secretAccessScope=issuer
   ```

2. **Create the Secret containing the SA JSON in the Issuer's namespace:**
   ```bash
   kubectl create secret generic stackit-tenant-a-auth \
     -n tenant-a \
     --from-file=sa.json=/path/to/tenant-a-sa.json
   ```

3. **Configure the Issuer:**
   `serviceAccountSecretNamespace` defaults to the `Issuer`'s namespace when omitted (and must match it if specified).
   ```yaml
   apiVersion: cert-manager.io/v1
   kind: Issuer
   metadata:
     name: letsencrypt-prod
     namespace: tenant-a
   spec:
     acme:
       server: https://acme-v02.api.letsencrypt.org/directory
       email: mail@example.com
       privateKeySecretRef:
         name: letsencrypt-prod
       solvers:
       - dns01:
           webhook:
             solverName: stackit
             groupName: acme.stackit.de
             config:
               projectId: <STACKIT PROJECT ID>
               serviceAccountSecretRef: stackit-tenant-a-auth
               serviceAccountSecretKey: sa.json
   ```

### Option B: Automatic SDK `DefaultAuth` (Environment Variables / Mounted SA Key)

When no explicit authentication field is set in the `Issuer`'s `config`, the webhook delegates credential discovery to the STACKIT Go SDK's `DefaultAuth` flow (checking Workload Identity Federation env vars, `STACKIT_SERVICE_ACCOUNT_KEY`, `STACKIT_SERVICE_ACCOUNT_KEY_PATH`, etc.). Because dynamic Secret lookup is not needed, you can disable Secret RBAC completely with `secretAccessScope=none`.

1. **Deploy the Webhook with Credentials Exposed as Environment Variables (or Mounted via `stackitSaAuthentication.enabled=true`):**
   ```bash
   kubectl create secret generic stackit-sa-authentication \
     -n cert-manager \
     --from-file=sa.json=/path/to/global-sa.json

   helm upgrade --install stackit-cert-manager-webhook stackit-cert-manager-webhook/stackit-cert-manager-webhook \
     --namespace cert-manager \
     --set stackitSaAuthentication.enabled=true \
     --set stackitSaAuthentication.secretAccessScope=none
   ```
   *(Enabling `stackitSaAuthentication.enabled=true` mounts the Secret at `/var/run/secrets/stackit/sa.json` and sets `STACKIT_SERVICE_ACCOUNT_KEY_PATH` automatically. Alternatively, pass SDK environment variables via `extraEnv`.)*

2. **Configure the `ClusterIssuer` (only `projectId` required):**
   ```yaml
   apiVersion: cert-manager.io/v1
   kind: ClusterIssuer
   metadata:
     name: letsencrypt-prod
   spec:
     acme:
       server: https://acme-v02.api.letsencrypt.org/directory
       email: mail@example.com
       privateKeySecretRef:
         name: letsencrypt-prod
       solvers:
       - dns01:
           webhook:
             solverName: stackit
             groupName: acme.stackit.de
             config:
               projectId: <STACKIT PROJECT ID>
   ```

### Option C: Explicit Static Service Account Key Path

If you mount one or more Service Account key files under `/var/run/secrets/stackit/` and want to select a specific file path in the `ClusterIssuer`/`Issuer`:

```yaml
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-prod
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: mail@example.com
    privateKeySecretRef:
      name: letsencrypt-prod
    solvers:
    - dns01:
        webhook:
          solverName: stackit
          groupName: acme.stackit.de
          config:
            projectId: <STACKIT PROJECT ID>
            serviceAccountKeyPath: /var/run/secrets/stackit/sa.json
```
*(Note: `serviceAccountKeyPath` must reside within `/var/run/secrets/stackit/`.)*

### Option D: Workload Identity Federation (WIF)

If your cluster supports Workload Identity Federation (e.g., SKE clusters), you can avoid long-lived keys by projecting a short-lived federated token into the webhook container.

1. **Annotate the Webhook ServiceAccount and Disable Secret RBAC:**
   ```yaml
   # values.yaml
   serviceAccount:
     annotations:
       workload-identity.stackit.cloud/service-account-email: "my-sa@sa.stackit.cloud"

   stackitSaAuthentication:
     secretAccessScope: none
   ```

2. **Configure the `ClusterIssuer`:**
   Either omit auth fields to let `DefaultAuth` pick up the projected WIF environment automatically, or explicitly enforce WIF:
   ```yaml
   apiVersion: cert-manager.io/v1
   kind: ClusterIssuer
   metadata:
     name: letsencrypt-prod
   spec:
     acme:
       server: https://acme-v02.api.letsencrypt.org/directory
       email: mail@example.com
       privateKeySecretRef:
         name: letsencrypt-prod
       solvers:
       - dns01:
           webhook:
             solverName: stackit
             groupName: acme.stackit.de
             config:
               projectId: <STACKIT PROJECT ID>
               useWorkloadIdentityFederation: true
   ```

## Config Options

### Solver `config` Fields (`Issuer` / `ClusterIssuer`)

| Key | Type | Default | Description |
|-----|------|--------|-------------|
| `projectId` | string | `""` | **Required.** The unique identifier for the STACKIT project. |
| `apiBasePath` | string | `"https://dns.api.stackit.cloud"` | Base URL for the STACKIT DNS API. Must match `https://dns.api([.<region>]).stackit.cloud`. |
| `serviceAccountSecretRef` | string | `""` | Name of the Kubernetes `Secret` containing the STACKIT Service Account JSON. |
| `serviceAccountSecretKey` | string | `"sa.json"` | Key within the `Secret` mapped to the Service Account JSON (when `serviceAccountSecretRef` is set). |
| `serviceAccountSecretNamespace` | string | `<issuer-or-webhook-namespace>` | Namespace of the `Secret`. Must match the `Issuer` namespace (or webhook namespace for `ClusterIssuer`). |
| `serviceAccountKeyPath` | string | `""` | File path to a mounted Service Account JSON key inside the webhook container. Must be within `/var/run/secrets/stackit/`. |
| `useWorkloadIdentityFederation` | bool | `false` | Explicitly enforces STACKIT Workload Identity Federation authentication. |
| `acmeTxtRecordTTL` | int32 | `60` | TTL (in seconds) for the ACME TXT challenge record. |

### Webhook Pod Environment Variables (via Helm `extraEnv`)

| Variable | Description |
|---|---|
| `STACKIT_SERVICE_ACCOUNT_BASE_URL` | Optional custom token endpoint URL for exchanging Service Account assertions (replaces the removed `serviceAccountBaseUrl` solver config field). |
| `STACKIT_SERVICE_ACCOUNT_KEY` / `STACKIT_SERVICE_ACCOUNT_KEY_PATH` | Used automatically by SDK `DefaultAuth` when no explicit solver auth option is set. |
| `STACKIT_FEDERATED_TOKEN_FILE` / `STACKIT_SERVICE_ACCOUNT_EMAIL` | Used automatically by SDK `DefaultAuth` / Workload Identity Federation. |

## Migrating from v0.x to v1.0.0

### Summary of Breaking Changes

1. **Cluster-wide `Secret` RBAC removed by default (`stackitSaAuthentication.secretAccessScope: webhook`):**
   - Previously, the Helm chart always created a `ClusterRole` able to read `Secrets` across all namespaces.
   - In `v1.0.0`, the default `secretAccessScope` is `webhook` (creates a namespace-scoped `Role` in the webhook's namespace only).
   - **Action required:**
     - If you have namespaced `Issuer`s in other namespaces using `serviceAccountSecretRef`, upgrade Helm with `--set stackitSaAuthentication.secretAccessScope=issuer`.
     - If you use WIF, `DefaultAuth`, or a mounted static Service Account key (`stackitSaAuthentication.enabled=true`), you can disable Secret RBAC completely with `--set stackitSaAuthentication.secretAccessScope=none`.
2. **Cross-namespace `Secret` references are blocked:**
   - A namespaced `Issuer` can no longer read a `Secret` from a different namespace (including `cert-manager`). Move the `Secret` into the `Issuer`'s own namespace (with `secretAccessScope=issuer`), or switch to a `ClusterIssuer` with its `Secret` in `cert-manager`.
3. **Legacy static Bearer Token authentication removed (`authTokenSecretRef`, `authTokenSecretKey`, `authTokenSecretNamespace`, `STACKIT_AUTH_TOKEN`):**
   - Long-lived static bearer tokens are no longer supported by STACKIT IAM/DNS and have been removed.
   - **Action required:** Create a STACKIT Service Account key JSON (or configure SKE Workload Identity Federation) and replace `authTokenSecretRef` with `serviceAccountSecretRef` (or `DefaultAuth` / WIF).
4. **`serviceAccountBaseUrl` removed from `Issuer` solver `config`:**
   - To prevent credential assertion exfiltration via untrusted `Issuer`s, custom token endpoints can no longer be set in `Issuer` YAML.
   - **Action required:** Remove `serviceAccountBaseUrl` from your `Issuer`/`ClusterIssuer` manifests. If you need a custom token endpoint (e.g., internal testing), set `STACKIT_SERVICE_ACCOUNT_BASE_URL` on the webhook Deployment via Helm `extraEnv`:
     ```yaml
     extraEnv:
       - name: STACKIT_SERVICE_ACCOUNT_BASE_URL
         value: "https://service-account.api.stackit.cloud/token"
     ```
5. **Strict validation on `apiBasePath` and `serviceAccountKeyPath`:**
   - `apiBasePath` only accepts official STACKIT DNS endpoints (`https://dns.api.stackit.cloud` or `https://dns.api.<region>.stackit.cloud`).
   - `serviceAccountKeyPath` must be under `/var/run/secrets/stackit/`.
6. **Default `acmeTxtRecordTTL` lowered:**
   - Default `acmeTxtRecordTTL` changed from `600` to `60` seconds (introduced in `v0.5.0`).
7. **Webhook PKI private key rotation policy changed to `Always`:**
   - The webhook's internal CA and serving certificates (`pki.caCertificate` and `pki.servingCertificate`) now default `privateKey.rotationPolicy` to `Always` (previously `Never`) and expose configurable `duration`, `renewBefore`, and `privateKey.rotationPolicy` under `pki.*` in Helm `values.yaml`.

### Before & After Migration Examples

#### Case 1: Migrating from `authTokenSecretRef` (v0.x) to Dynamic SA Key (v1.0.0)

```yaml
# BEFORE (v0.x - no longer supported)
config:
  projectId: <STACKIT PROJECT ID>
  authTokenSecretRef: stackit-cert-manager-webhook
  authTokenSecretKey: auth-token
  authTokenSecretNamespace: my-namespace

# AFTER (v1.0.0 - with Helm `--set stackitSaAuthentication.secretAccessScope=issuer` for tenant namespaces)
config:
  projectId: <STACKIT PROJECT ID>
  serviceAccountSecretRef: stackit-sa-secret   # Secret in `my-namespace` containing key `sa.json`
  serviceAccountSecretKey: sa.json
```

#### Case 2: Migrating Single-Tenant Mounted SA Key (v0.x → v1.0.0)

```yaml
# Helm values.yaml (v1.0.0)
stackitSaAuthentication:
  enabled: true
  secretAccessScope: none
  secretName: stackit-sa-authentication
  fileName: sa.json
```
```yaml
# ClusterIssuer solver config (v1.0.0 - DefaultAuth picks up mounted key automatically)
config:
  projectId: <STACKIT PROJECT ID>
```

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
TEST_ZONE_NAME="example.com" make test-e2e-conformance
```
Follow the comprehensive guide available [here](e2e_test/README.md).

### Kubernetes Integration (E2E) Testing:
Spins up a local Kind cluster, installs cert-manager, builds and deploys the webhook, and executes Kuttl integration tests covering both single-tenant (`DefaultAuth`) and multi-tenant (dynamic SA fetching & cross-tenant isolation) flows against Let's Encrypt Staging:

```bash
make test-e2e-local \
  PROJECT_ID="<your-project-id>" \
  ZONE_NAME="<your-test-zone>" \
  AUTH_KEY_PATH="<path-to-sa-key.json>"
```
Follow the comprehensive guide available [here](tests/e2e/README.md).

## Release Process Overview

1. Update `version` and `appVersion` in [`deploy/stackit/Chart.yaml`](./deploy/stackit/Chart.yaml) and regenerate chart documentation via `make helm-docs`.
2. Merge changes into `main` — `.github/workflows/publish-chart.yaml` automatically packages and publishes the updated Helm chart to GitHub Pages.
3. Push a semantic version tag (`vX.Y.Z`) on `main` — `.github/workflows/release.yaml` runs GoReleaser to build, sign (Cosign), generate SBOMs, and publish multi-arch container images (`ghcr.io/stackitcloud/stackit-cert-manager-webhook`) and GitHub Release artifacts.
