FROM golang:1.25.11-bookworm

ARG NODE_VERSION=24.14.0
ARG PNPM_VERSION=11.9.0
ARG SQLC_VERSION=1.29.0

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        ca-certificates \
        chromium \
        curl \
        git \
        make \
        python3 \
        sqlite3 \
        xz-utils \
    && rm -rf /var/lib/apt/lists/*

RUN curl -fsSLO "https://nodejs.org/dist/v${NODE_VERSION}/node-v${NODE_VERSION}-linux-x64.tar.xz" \
    && tar -C /usr/local --strip-components=1 -xJf "node-v${NODE_VERSION}-linux-x64.tar.xz" \
    && rm "node-v${NODE_VERSION}-linux-x64.tar.xz" \
    && corepack enable \
    && corepack prepare "pnpm@${PNPM_VERSION}" --activate

RUN go install "github.com/sqlc-dev/sqlc/cmd/sqlc@v${SQLC_VERSION}"

ENV PATH="/go/bin:${PATH}"

WORKDIR /workspaces/RayleaBot
