FROM gcr.io/distroless/static-debian11:nonroot

COPY stackit-cert-manager-webhook /stackit-cert-manager-webhook

ENTRYPOINT ["/stackit-cert-manager-webhook"]
