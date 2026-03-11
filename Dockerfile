FROM gcr.io/distroless/static-debian11:nonroot

# Buildx automatically populates this with the current architecture (e.g., "linux/amd64")
ARG TARGETPLATFORM

# Grab the binary from the architecture-specific folder
COPY ${TARGETPLATFORM}/stackit-cert-manager-webhook /stackit-cert-manager-webhook

ENTRYPOINT ["/stackit-cert-manager-webhook"]