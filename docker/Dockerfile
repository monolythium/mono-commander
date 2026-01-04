# Dockerfile for Monolythium Node
# Multi-stage build to minimize final image size

# Stage 1: Download and verify monod binary
FROM alpine:3.19 AS downloader

ARG MONOD_VERSION=latest
ARG TARGETOS=linux
ARG TARGETARCH=amd64

WORKDIR /tmp

# Install dependencies
RUN apk add --no-cache curl jq

# Download monod binary from GitHub releases
# If MONOD_VERSION=latest, fetch latest release tag
RUN if [ "$MONOD_VERSION" = "latest" ]; then \
      MONOD_VERSION=$(curl -s https://api.github.com/repos/monolythium/mono-core/releases/latest | jq -r .tag_name); \
    fi && \
    echo "Downloading monod ${MONOD_VERSION} for ${TARGETOS}/${TARGETARCH}" && \
    curl -L -o monod "https://github.com/monolythium/mono-core/releases/download/${MONOD_VERSION}/monod-${TARGETOS}-${TARGETARCH}" && \
    chmod +x monod

# Stage 2: Final runtime image
FROM debian:bookworm-slim

LABEL maintainer="Monolythium Team <team@monolythium.com>"
LABEL description="Monolythium Node - Ethereum-compatible Layer 1"

# Install runtime dependencies
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      ca-certificates \
      curl \
      jq && \
    rm -rf /var/lib/apt/lists/*

# Copy monod binary from downloader stage
COPY --from=downloader /tmp/monod /usr/local/bin/monod

# Set working directory
WORKDIR /root

# Expose ports
# P2P, RPC, REST, gRPC, EVM RPC, EVM WS
EXPOSE 26656 26657 1317 9090 8545 8546

# Volume for node data
VOLUME ["/root/.monod"]

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=60s --retries=3 \
  CMD curl -f http://localhost:26657/health || exit 1

# Entrypoint
ENTRYPOINT ["monod"]

# Default command
CMD ["start", "--home", "/root/.monod"]
