# Cert-Manager ACME DNS01 Webhook Solver for STACKIT DNS Manager

## testdata Directory

Copy the example Secret files, replacing $STACKIT_TOKEN with your STACKIT API
token:

```bash
$ export STACKIT_TOKEN=$(echo -n "<token>" | base64)
$ envsubst < testdata/stackit/secret.yaml.example | kubectl apply -f -
```
