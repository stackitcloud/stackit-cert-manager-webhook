# stackit-cert-manager-webhook

![Version: 0.3.0-alpha1](https://img.shields.io/badge/Version-0.3.0--alpha1-informational?style=flat-square) ![AppVersion: 1.0](https://img.shields.io/badge/AppVersion-1.0-informational?style=flat-square)

A Helm chart for Kubernetes

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| affinity | object | `{}` |  |
| certManager | object | `{"namespace":"cert-manager","serviceAccountName":"cert-manager"}` | Meta information of the cert-manager itself. |
| certManager.namespace | string | `"cert-manager"` | namespace where the webhook should be installed. Cert-Manager and the webhook should be in the same namespace. |
| certManager.serviceAccountName | string | `"cert-manager"` | service account name for the cert-manager. |
| fullnameOverride | string | `""` | Fullname override of the webhook. |
| groupName | string | `"acme.stackit.de"` | The GroupName here is used to identify your company or business unit that created this webhook. Therefore, it should be acme.stackit.de. |
| image | object | `{"pullPolicy":"IfNotPresent","repository":"ghcr.io/stackitcloud/stackit-cert-manager-webhook","tag":"latest"}` | Image information for the webhook. |
| image.pullPolicy | string | `"IfNotPresent"` | pull policy of the image. |
| image.repository | string | `"ghcr.io/stackitcloud/stackit-cert-manager-webhook"` | repository of the image. |
| image.tag | string | `"latest"` | tag of the image. |
| nameOverride | string | `""` | Webhook configuration. |
| nodeSelector | object | `{}` | Node selector for the webhook. |
| podSecurityContext.runAsGroup | int | `1000` |  |
| podSecurityContext.runAsNonRoot | bool | `true` |  |
| podSecurityContext.runAsUser | int | `1000` |  |
| replicaCount | int | `1` | Replicas for the webhook. Since it is a stateless application server that sends requests you can increase the number as you want. Most of the time however, 1 replica is enough. |
| resources | object | `{}` | Kubernetes resources for the webhook. Usually limits.cpu=100m, limits.memory=128Mi, requests.cpu=100m, requests.memory=128Mi is enough for the webhook. |
| securityContext.allowPrivilegeEscalation | bool | `false` |  |
| securityContext.capabilities.drop[0] | string | `"ALL"` |  |
| service | object | `{"port":443,"type":"ClusterIP"}` | Configuration for the webhook service. |
| service.port | int | `443` | port of the service. |
| service.type | string | `"ClusterIP"` | type of the service. |
| stackitSaAuthentication | object | `{"enabled":false,"fileName":"sa.json","mountPath":"/var/run/secrets/stackit","secretName":"stackit-sa-authentication"}` | Configuration for the stackit service account keys. |
| stackitSaAuthentication.enabled | bool | `false` | enabled flag for the stackit service account keys. |
| stackitSaAuthentication.fileName | string | `"sa.json"` | key of the service account key in the secret. Which will be later be used to load in keys in the pod as well. |
| stackitSaAuthentication.mountPath | string | `"/var/run/secrets/stackit"` | Path where the secret will be mounted in the pod. |
| stackitSaAuthentication.secretName | string | `"stackit-sa-authentication"` | secret where the service account key is stored. Should be in the same namespace as the webhook since it will be mounted into the pod. |
| tolerations | list | `[]` | Tolerations for the webhook. |

