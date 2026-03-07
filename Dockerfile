# Stage 1: Build Go binary
FROM golang:1.25-bookworm AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN HTTPS_PROXY="" HTTP_PROXY="" http_proxy="" https_proxy="" go mod download

COPY . .
RUN HTTPS_PROXY="" HTTP_PROXY="" http_proxy="" https_proxy="" CGO_ENABLED=0 go build -o /aisupervisor ./cmd/aisupervisor

# Stage 2: Runtime environment
FROM debian:bookworm-slim

RUN HTTPS_PROXY="" HTTP_PROXY="" http_proxy="" https_proxy="" \
    apt-get update && apt-get install -y --no-install-recommends \
    tmux \
    git \
    curl \
    ca-certificates \
    openssh-client \
    gnupg \
    && rm -rf /var/lib/apt/lists/*

# Install Node.js 20 LTS
RUN HTTPS_PROXY="" HTTP_PROXY="" http_proxy="" https_proxy="" \
    bash -c 'curl -fsSL https://deb.nodesource.com/setup_20.x | bash -' \
    && apt-get install -y --no-install-recommends nodejs \
    && rm -rf /var/lib/apt/lists/*

# Install Claude Code CLI
RUN HTTPS_PROXY="" HTTP_PROXY="" http_proxy="" https_proxy="" \
    npm install -g @anthropic-ai/claude-code

# Copy aisupervisor binary
COPY --from=builder /aisupervisor /usr/local/bin/aisupervisor

# Copy entrypoint
COPY docker/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Clear proxy env vars for runtime
ENV http_proxy="" https_proxy="" HTTP_PROXY="" HTTPS_PROXY=""

# OPENAI_API_KEY is passed via docker-compose environment

WORKDIR /workspace

ENTRYPOINT ["/entrypoint.sh"]
