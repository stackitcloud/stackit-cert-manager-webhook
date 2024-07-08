# STACKIT Webhook Integration for Cert Manager

[![GoTemplate](https://img.shields.io/badge/go/template-black?logo=go)](https://github.com/golang-standards/project-layout)
[![Go](https://img.shields.io/badge/go-1.21.0-blue?logo=go)](https://golang.org/)
[![Helm](https://img.shields.io/badge/helm-3.12.3-blue?logo=helm)](https://helm.sh/)
[![Kubernetes](https://img.shields.io/badge/kubernetes-1.26.0-blue?logo=kubernetes)](https://kubernetes.io/)
[![Cert Manager](https://img.shields.io/badge/cert--manager-1.11.0-blue?logo=cert-manager)](https://cert-manager.io/)
[![Releases](https://img.shields.io/github/v/release/stackitcloud/stackit-cert-manager-webhook?include_prereleases)](https://github.com/stackitcloud/stackit-cert-manager-webhook/releases)
[![CI](https://github.com/stackitcloud/stackit-cert-manager-webhook/actions/workflows/main.yml/badge.svg)](https://github.com/stackitcloud/stackit-cert-manager-webhook/actions/workflows/main.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/stackitcloud/stackit-cert-manager-webhook)](https://goreportcard.com/report/github.com/stackitcloud/stackit-cert-manager-webhook)

Facilitate a webhook integration for leveraging the STACKIT DNS alongside
its [API](https://docs.api.stackit.cloud/documentation/dns/version/v1) to act as a DNS01
ACME Issuer with [cert-manager](https://cert-manager.io/docs/).

## Installation

```bash
helm repo add stackit-cert-manager-webhook https://stackitcloud.github.io/stackit-cert-manager-webhook
helm install stackit-cert-manager-webhook --namespace cert-manager stackit-cert-manager-webhook/stackit-cert-manager-webhook
```

## Usage

1. ***Initiation of STACKIT Authentication Token Secret:***
    ```bash
    kubectl create secret generic stackit-cert-manager-webhook \
      --namespace=cert-manager \
      --from-literal=auth-token=<STACKIT AUTH TOKEN>
    ```
   Or alternatively we can utilize the STACKIT service account path authentication:
      ```
    kubectl create secret generic stackit-sa-authentication -n cert-manager \
        --from-literal=sa.json='{
      "id": "4e1fe486-b463-4bcd-9210-288854268e34",
      "publicKey": "-----BEGIN PUBLIC KEY-----\nPUBLIC_KEY\n-----END PUBLIC KEY-----",
      "createdAt": "2024-04-02T13:12:17.678+00:00",
      "validUntil": "2024-04-15T22:00:00.000+00:00",
      "keyType": "USER_MANAGED",
      "keyOrigin": "GENERATED",
      "keyAlgorithm": "RSA_2048",
      "active": true,
      "credentials": {
        "kid": "kid",
        "iss": "iss",
        "sub": "sub",
        "aud": "aud",
        "privateKey": "-----BEGIN PRIVATE KEY-----\nPRIVATE-KEY==\n-----END PRIVATE KEY-----"
      }
    }'
    ```
   You now need to adjust the deployment via helm to use the secret:
    ```bash
    helm upgrade stackit-cert-manager-webhook \
      --namespace cert-manager \
      stackit-cert-manager-webhook/stackit-cert-manager-webhook \
     --set stackitSaAuthentication.enabled=true
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
              groupName: acme.stackit.de
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
              groupName: acme.stackit.de
              config:
                projectId: <STACKIT PROJECT ID>
                authTokenSecretNamespace: default
    ```
   *Note:* Ensure the creation of an authentication token secret within the namespace linked to the issuer.
   The secret must be vested with permissions to access zones in the stipulated project configuration.
3. ***Demonstration of Ingress Integration with Wildcard SSL/TLS Certificate Generation***   
   Given the preceding configuration, it is possible to exploit the capabilities of the Issuer or ClusterIssuer to
   dynamically produce wildcard SSL/TLS certificates in the following manner:
    ```yaml
    apiVersion: cert-manager.io/v1
    kind: Certificate
    metadata:
      name: wildcard-example
      namespace: default
    spec:
      secretName: wildcard-example-tls
      issuerRef:
        name: letsencrypt-prod
        kind: Issuer
      commonName: '*.example.runs.onstackit.cloud' # project must be the owner of this zone
      duration: 8760h0m0s
      dnsNames:
        - example.runs.onstackit.cloud
        - '*.example.runs.onstackit.cloud'
    ---
    apiVersion: networking.k8s.io/v1
    kind: Ingress
    metadata:
      name: app-ingress
      namespace: default
      annotations:
        ingress.kubernetes.io/rewrite-target: /
        kubernetes.io/ingress.class: "nginx"
    spec:
      rules:
        - host: "app.example.runs.onstackit.cloud"
          http:
            paths:
              - path: /
                pathType: Prefix
                backend:
                  service:
                    name: webapp
                    port:
                      number: 80
      tls:
        - hosts:
            - "app.example.runs.onstackit.cloud"
          secretName: wildcard-example-tls
    ```

## Config Options

The following table delineates the configuration options available for the STACKIT Cert Manager Webhook:

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
          groupName: acme.stackit.de
          config:
            projectId: string
            apiBasePath: string
            authTokenSecretRef: string
            authTokenSecretKey: string
            authTokenSecretNamespace: string
            serviceAccountKeyPath: string
            acmeTxtRecordTTL: int64
```

- projectId: The unique identifier for the STACKIT project.
- apiBasePath: The base path for the STACKIT DNS API. (Default: https://dns.api.stackit.cloud)
- authTokenSecretRef: The reference to the secret containing the STACKIT authentication token. (Default:
  stackit-cert-manager-webhook)
- authTokenSecretKey: The key within the secret containing the STACKIT authentication token. (Default: auth-token)
- authTokenSecretNamespace: The namespace of the secret containing the STACKIT authentication token. (Default: cert-manager)
- serviceAccountKeyPath: The path to the service account key file. The file must be mounted into the container.
- acmeTxtRecordTTL: The TTL for the ACME TXT record. (Default: 600)

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

To release a new version of the Helm chart, one must meticulously update the version delineation in the
[Chart.yaml](./deploy/stackit/Chart.yaml). Post this modification, initiate a new release to encompass these changes.
