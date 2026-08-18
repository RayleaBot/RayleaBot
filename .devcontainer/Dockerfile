FROM python:3.14.7-bookworm AS python-runtime

FROM golang:1.26.6-bookworm

ARG NODE_VERSION=26.7.0
ARG NPM_VERSION=11.19.0
ARG COREPACK_VERSION=0.35.0
ARG PNPM_VERSION=11.22.0
ARG SQLC_VERSION=1.31.1

COPY --from=python-runtime /usr/local /usr/local

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        ca-certificates \
        chromium \
        curl \
        git \
        make \
        sqlite3 \
        xz-utils \
    && rm -rf /var/lib/apt/lists/*

RUN curl -fsSLO "https://nodejs.org/dist/v${NODE_VERSION}/node-v${NODE_VERSION}-linux-x64.tar.xz" \
    && tar -C /usr/local --strip-components=1 -xJf "node-v${NODE_VERSION}-linux-x64.tar.xz" \
    && rm "node-v${NODE_VERSION}-linux-x64.tar.xz" \
    && test "$(node --version)" = "v${NODE_VERSION}" \
    && test "$(npm --version)" = "${NPM_VERSION}" \
    && npm install --global "corepack@${COREPACK_VERSION}" \
    && corepack enable \
    && corepack prepare "pnpm@${PNPM_VERSION}" --activate \
    && test "$(python --version)" = "Python 3.14.7"

RUN go install "github.com/sqlc-dev/sqlc/cmd/sqlc@v${SQLC_VERSION}"

ENV PATH="/go/bin:${PATH}"

WORKDIR /workspaces/RayleaBot
