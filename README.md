# STACKIT Webhook Integration for Cert Manager
[![GoTemplate](https://img.shields.io/badge/go/template-black?logo=go)](https://github.com/golang-standards/project-layout)
[![Go](https://img.shields.io/badge/go-1.21.0-blue?logo=go)](https://golang.org/)
[![Helm](https://img.shields.io/badge/helm-3.12.3-blue?logo=helm)](https://helm.sh/)
[![Kubernetes](https://img.shields.io/badge/kubernetes-1.28.0-blue?logo=kubernetes)](https://kubernetes.io/)
[![Cert Manager](https://img.shields.io/badge/cert--manager-1.12.3-blue?logo=cert-manager)](https://cert-manager.io/)
[![Releases](https://img.shields.io/github/v/release/stackitcloud/stackit-cert-manager-webhook?include_prereleases)](https://github.com/stackitcloud/stackit-cert-manager-webhook/releases)
[![CI](https://github.com/stackitcloud/stackit-api-manager-cli/actions/workflows/main.yml/badge.svg)](https://github.com/stackitcloud/stackit-cert-manager-webhook/actions/workflows/main.yml)
[![Semgrep](https://github.com/stackitcloud/stackit-api-manager-cli/actions/workflows/semgrep.yml/badge.svg)](https://github.com/stackitcloud/stackit-cert-manager-webhook/actions/workflows/semgrep.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/stackitcloud/stackit-api-manager-cli)](https://goreportcard.com/report/github.com/stackitcloud/stackit-cert-manager-webhook)

Facilitate a webhook integration for leveraging the STACKIT DNS alongside 
its [API](https://docs.api.stackit.cloud/documentation/dns/version/v1) to act as a DNS01 
ACME Issuer with [cert-manager](https://cert-manager.io/docs/).

## Installation
```bash
helm install stackit-cert-manager-webhook \
  --namespace cert-manager \
  https://github.com/stackitcloud/stackit-cert-manager-webhook/releases/download/v0.1.0/stackit-cert-manager-webhook-v0.1.0.tgz
```

## Usage
1. ***Initiation of STACKIT Authentication Token Secret:***
    ```bash
    kubectl create secret generic stackit-cert-manager-webhook \
      --namespace=cert-manager \
      --from-literal=auth-token=<STACKIT AUTH TOKEN>
    ```

2. ***Configuration of ClusterIssuer/Issuer:***   
For scenarios wherein zones and record sets are encapsulated within a singular project, utilize a ClusterIssuer:
    ```yaml
    apiVersion: cert-manager.io/v1
    kind: ClusterIssuer
    metadata:
      name: letsencrypt-prod
    spec:
      acme:
        server: https://acme-v02.api.letsencrypt.org/directory
        email: example@example.com # Replace this with your email address
        privateKeySecretRef:
          name: letsencrypt-prod
        solvers:
        - dns01:
          webhook:
            solverName: stackit
            groupName: stackit.de
            config:
              projectId: <STACKIT PROJECT ID>
    ```

    For diverse project architectures where zones are spread across varying projects, necessitating distinct 
    authentication tokens per project, the Issuer configuration becomes pertinent. This approach inherently 
    tethers namespaces to individual projects.
    ```bash
    kubectl create secret generic stackit-cert-manager-webhook \
      --namespace=default \
      --from-literal=auth-token=<STACKIT AUTH TOKEN>
    ```
    ```yaml
    apiVersion: cert-manager.io/v1
    kind: Issuer
    metadata:
      name: letsencrypt-prod
      namespace: default
    spec:
      acme:
        server: https://acme-v02.api.letsencrypt.org/directory
        email: example@example.com # Replace this with your email address
        privateKeySecretRef:
          name: letsencrypt-prod
        solvers:
        - dns01:
          webhook:
            solverName: stackit
            groupName: stackit.de
            config:
              projectId: <STACKIT PROJECT ID>
              authTokenSecretNamespace: default
    ```
    *Note:* Ensure the creation of an authentication token secret within the namespace linked to the issuer. 
    The secret must be vested with permissions to access zones in the stipulated project configuration.

## Test Procedures
- Unit Testing:
    ```bash
    make test
    ```

- Unit Testing with Coverage Analysis:
    ```bash
    make coverage
    ```

- Linting:
    ```bash
    make lint
    ```

- End-to-End Testing Workflow:  
Follow the comprehensive guide available [here](e2e_test/README.md).
