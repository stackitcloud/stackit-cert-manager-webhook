# End-to-End (E2E) Integration Testing

This directory contains the Kuttl-based integration test suite for the STACKIT `cert-manager` webhook.

The E2E pipeline creates a local [Kind](https://kind.sigs.k8s.io/) cluster, installs `cert-manager`, deploys the webhook image built from source, and executes assertions against real Let's Encrypt Staging ACME challenges.

---

## Test Cases

The test suite consists of the following sequential test scenarios:

1. **`record-lifecycle`**: Verifies the creation, propagation check, certificate issuance, and cleanup of a standard single domain certificate (`e2e-cert.<ZONE_NAME>`).
2. **`wildcard-certificate`**: Verifies concurrent challenge handling for a wildcard certificate request covering both base and wildcard domains (`*.e2e-wildcard.<ZONE_NAME>` and `e2e-wildcard.<ZONE_NAME>`).

---

## Prerequisites

To run local E2E tests, ensure the following tools are installed:

- **Docker**
- **Go** 
- **Kind** 
- **Kubectl**
- **Helm** 
- **Kuttl** (`kubectl-kuttl`)
- **Dig** (`dnsutils` package)

---

## Running E2E Tests Locally

1. Prepare a STACKIT Service Account key JSON file with permissions to manage DNS records in your target test zone.
2. Export required parameters and execute the Makefile target:

```bash
make test-e2e-local \
  PROJECT_ID="<your-project-id>" \
  ZONE_NAME="<your-test-zone>" \
  AUTH_KEY_PATH="<path-to-sa-key.json>"
```

## Note on Corporate Proxies & VPNs

When running tests locally on a corporate machine behind proxies or VPNs, cert-manager's propagation self-checks may timeout due to DNS interception or blocked outbound UDP port 53 traffic.

### Solution
If you encounter `dial tcp ...:53 i/o timeout` errors during cert-manager propagation checks, run `make test-e2e-local` in an environment that is unaffected by these network interceptions.
