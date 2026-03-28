# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What is RayleaBot

A self-hosted chatbot framework with a plugin system, targeting personal developers and GitHub open-source collaborators. Polyglot monorepo: Go server, Vue 3 web UI, Electron desktop launcher.

## Instruction Precedence

`docs/RayleaBot机器人项目规划.md > contracts/ > fixtures/examples > code`

- `contracts/` is the single source of truth for all external interfaces, schemas, error codes, and release metadata.
- If Markdown docs conflict with `contracts/`, `contracts/` wins — fix the docs in the same change.
- No implementation without a contract (or at minimum a skeleton with explicit `TODO`).

## Build & Test Commands

### Server (Go 1.25.8) — run from `server/`

```bash
go build ./cmd/raylea-server    # build
go test ./...                    # all tests
go test ./internal/auth/...      # single package
```

### Web (Node 24.14.0, pnpm 10.32.1) — run from `web/`

```bash
pnpm install --frozen-lockfile
pnpm dev          # dev server
pnpm build        # production build
pnpm test         # unit tests (vitest, scaffold only)
pnpm test:e2e     # e2e (playwright, scaffold only)
```

### Launcher (Electron 41.1.0, Node 24.14.0, pnpm 10.32.1) — run from `launcher/`

```bash
pnpm install --frozen-lockfile
pnpm test
pnpm build
```

## Contract-First Workflow

Any change touching protocol, schema, state machines, config, database, plugin install, Web API, WebSocket, error codes, or migrations must include all four:

1. Contract update (`contracts/`)
2. Fixture update (`fixtures/`)
3. Test update
4. Implementation update

No "code first, contract later" — CI gates enforce this.

## Architecture Overview

### Server (`server/internal/`)

| Package | Role |
|---------|------|
| `adapter` | OneBot11 reverse WebSocket client, state machine, event intake, CQ/segment parsing, internal API calls (get_login_info etc.), identity cache |
| `app` | Application bootstrap, HTTP router assembly (chi v5), lifecycle, plugin lifecycle controller |
| `auth` | HMAC-SHA256 session tokens, bootstrap admin, persistence |
| `bridge` | Adapter → Dispatch event routing, observability counters, WebSocket subscriber notification |
| `cli` | CLI subcommands: `reset-admin`, `doctor`, `cleanup`, `migrate`, `backup`, `restore` |
| `command` | Command prefix parser (longest-prefix-first matching, command/args extraction) |
| `config` | YAML parsing + JSON schema validation |
| `console` | Plugin stderr/system capture, ring buffer, redaction |
| `dispatch` | Multi-plugin fan-out delivery, per-plugin async queues, directed command routing, subscription filtering, zero-gap reload |
| `health` | Liveness (`/healthz`) and readiness (`/readyz`) probes |
| `httpapi` | HTTP handler wiring |
| `logging` | Structured slog, stream capture for management API |
| `permission` | Chat-side permission checker (super_admin bypass → blacklist → role check → cooldown), blacklist SQLite repository |
| `plugins` | Catalog, discovery, manifest validation, install/uninstall coordination |
| `redact` | Sensitive field masking |
| `runtime` | Plugin subprocess lifecycle, JSONL protocol, crash backoff |
| `scheduler` | Cron-based job scheduling with SQLite persistence and tick-loop execution |
| `schema` | JSON schema loader/validator |
| `secrets` | Key-value secret store backed by SQLite |
| `storage` | SQLite (WAL mode, read pool max 4 / write pool max 1), numbered migrations with SHA256 checksums |
| `tasks` | In-memory task registry with SQLite persistence and cross-restart hydration |

### Data Flow

```
OneBot server ←WS→ Adapter → Bridge → Dispatch (fan-out) → Plugin Runtimes (subprocess, JSONL)
                                          ↑ command parse + permission check
Management UI ←HTTP/WS→ httpapi ──────────┘
```

### Plugin System

- Plugins are subprocesses (Python 3.12.13 or Node.js 24.14.0)
- Communication: JSONL over stdin/stdout
- Protocol messages: `init`, `event`, `shutdown` (platform→plugin); `init_ack`, `action`, `result`, `error`, `ping`, `pong` (plugin→platform)
- Frozen actions: `message.send`, `message.reply`, `message.send_image`
- Event kinds: `onebot11.message` (with segments), `onebot11.notice` (member increase/decrease)
- Lifecycle: `stopped → starting → running → stopping → stopped` with crash-backoff supervision
- Dependencies: per-plugin `.venv/` (Python) or `npm install` (Node.js)
- Official SDKs: `sdk/python/rayleabot` (Python), `sdk/nodejs/@rayleabot/sdk` (Node.js)
- Built-in plugins: `plugins/builtin/help/`

### Storage

SQLite via `modernc.org/sqlite` (pure Go). WAL mode, numbered SQL migrations in `internal/storage/migrations/` with SHA256 checksums. 10 migrations covering: `schema_migrations`, `auth_bootstrap_state`, `admin_sessions`, `plugin_instances`, `plugin_packages`, `plugin_grants`, `tasks`, `secret_store`, `scheduler_jobs`, `blacklist_entries`.

### Auth

Bearer token (`Authorization: Bearer <token>`) or `session_token` query param (WebSocket). Sliding window renewal. One-time bootstrap for first admin.

## Key Contracts (all in `contracts/`)

| File | What it defines |
|------|-----------------|
| `config.user.schema.json` | User config YAML schema |
| `error-codes.yaml` | Unified error catalog |
| `web-api.openapi.yaml` | HTTP management API |
| `websocket-events.yaml` | Management WebSocket events |
| `plugin-info.schema.json` | Plugin manifest (`info.json`) |
| `plugin-protocol.schema.json` | Plugin JSONL protocol |
| `release-manifest.schema.json` | Release metadata |
| `cli-commands.yaml` | CLI model (skeleton, all TODO) |

## CI Workflows

- `contracts.yml` — validates contract structure, fixture existence, parsability, cross-references
- `lint.yml` — baseline version pinning, required files/directories

## Git Commit Rules

Conventional Commits: `<type>[scope][!]: <description>`

Types: `feat`, `fix`, `refactor`, `perf`, `docs`, `test`, `build`, `ci`, `chore`
Scopes: `server`, `contracts`, `fixtures`, `docs`, `web`, `launcher`, `auth`, `adapter`, `bridge`, `runtime`, `storage`, `dispatch`, `command`, `permission`

One logical change per commit. Split mixed concerns (contracts vs implementation vs migration vs docs). Prefer a short **subject line**; target **<= 72 characters for the subject line when practical**, but do **not** hard-wrap or forcibly reflow text to satisfy this.

- Format commit bodies for scan readability:
  - use short paragraphs or bullet lists
  - keep one bullet per concern
  - do **not** hard-wrap lines mechanically at 72 characters; wrap only when it improves readability
  - avoid awkward line breaks inside a phrase or sentence
  - put validation in a separate final paragraph or `Validation:` block

- Mark breaking changes with either:
  - `!` after the type/scope, or
  - a `BREAKING CHANGE:` footer

## Cross-Layer Rules (Hard Boundaries)

- Adapter must not write to state store directly
- Launcher must not duplicate Web business logic
- Web UI must not infer state from logs
- Plugins must not bypass Capability checks or read `config/user.yaml` directly
- All layers must use the same state names, error codes, and task states (defined in contracts)

## Documentation Style

Output final-form prose only. No edit-trail language (`不再`, `已改为`, `原来`, `之前`, etc.). No phase/step markers in non-doc areas.
