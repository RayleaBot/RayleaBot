# Technology Decisions

RayleaBot keeps the current server stack unless a change removes a concrete limitation that cannot be solved within the existing stack.

## Current Baseline

| Area | Decision |
| --- | --- |
| Server language | Go |
| HTTP router | chi |
| Storage | SQLite |
| SQL access | sqlc |
| Logging | slog |
| Metrics | Prometheus-compatible registry |
| Browser automation | chromedp |

## Decision Rules

New dependencies or replacement tools must record:

- the concrete problem being solved;
- why the existing stack is insufficient;
- whether the change introduces a parallel stack;
- the rollback path;
- the impact on CI, release packaging, lockfiles, fixtures, and generated files.

## Current Evaluation Areas

| Area | Current direction |
| --- | --- |
| Database migration tooling | Keep the current snapshot plus legacy migration runner while drift tests stay effective; evaluate goose, golang-migrate, or Atlas only with a small spike. |
| OpenAPI implementation | Keep strict contract validation and generated type checks; evaluate server-side OpenAPI code generation only if handler drift continues. |
| Secret storage | Keep sealed SQLite-backed secrets; evaluate environment keys, OS keychain, or external KMS when deployment targets need external key custody. |
| Architecture gates | Keep repository-specific structure tests and budget files because they encode local package boundaries better than a generic linter. |
