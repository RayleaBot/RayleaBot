FROM python:3.14.8-bookworm AS python-runtime

FROM golang:1.27.1-bookworm

COPY --from=python-runtime /usr/local /usr/local
COPY .tool-versions scripts/read-tool-versions.sh /tmp/toolchain/
COPY scripts/requirements.txt /tmp/toolchain/requirements.txt

RUN python -m pip install --disable-pip-version-check --no-cache-dir -r /tmp/toolchain/requirements.txt

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

RUN eval "$(bash /tmp/toolchain/read-tool-versions.sh /tmp/toolchain/.tool-versions)" \
    && test "$(go version | cut -d ' ' -f 3)" = "go${golang}" \
    && test "$(python --version)" = "Python ${python}" \
    && curl -fsSLO "https://nodejs.org/dist/v${nodejs}/node-v${nodejs}-linux-x64.tar.xz" \
    && tar -C /usr/local --strip-components=1 -xJf "node-v${nodejs}-linux-x64.tar.xz" \
    && rm "node-v${nodejs}-linux-x64.tar.xz" \
    && test "$(node --version)" = "v${nodejs}" \
    && test "$(npm --version)" = "${npm}" \
    && npm install --global "corepack@${corepack}" \
    && corepack enable \
    && corepack prepare "pnpm@${pnpm}" --activate \
    && go install "github.com/sqlc-dev/sqlc/cmd/sqlc@v${sqlc}"

ENV PATH="/go/bin:${PATH}"

WORKDIR /workspaces/RayleaBot
