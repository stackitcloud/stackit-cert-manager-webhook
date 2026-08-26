# Cert-Manager ACME DNS01 Webhook Solver for STACKIT DNS

## testdata Directory

Copy the example Secret file, replacing `$STACKIT_SA_KEY` with your STACKIT Service Account JSON key:

```bash
$ export STACKIT_SA_KEY=$(cat /path/to/sa.json | base64 -w 0)
$ envsubst < testdata/stackit/secret.yaml.example | kubectl apply -f -
```