# Base image for running user Go code in sandbox
FROM golang:1.26-alpine@sha256:30b1e546a9c7fa3b747c2bdba3a0c7bdf3a02ebb4c5b8405d2f948d7e8aac4ec

LABEL org.opencontainers.image.authors="Felix Geelhaar"

# Install additional tools for code analysis
RUN apk add --no-cache git

# Pre-install useful Go tools
RUN go install golang.org/x/tools/cmd/goimports@latest && \
    go install honnef.co/go/tools/cmd/staticcheck@latest && \
    go install golang.org/x/vuln/cmd/govulncheck@latest

# Create non-root user for running code
RUN adduser -D -u 1000 runner

# Set up workspace
WORKDIR /sandbox
RUN chown runner:runner /sandbox

USER runner

# Default shell for executing commands
ENTRYPOINT ["/bin/sh", "-c"]
