---
name: glue-coding
description: Use when planning or coding RayleaBot implementation changes in server, web, launcher, storage, render, or dependency selection, especially when deciding whether to add a new library, framework, service, or cross-surface pattern. Prefer existing repo code, the frozen stack, standard library, and thin glue over parallel stacks or fresh reimplementation.
---

# Glue Coding

This skill is a reusable workflow. Repository truth still lives in root/local `AGENTS.md`, `contracts/`, and the engineering docs they reference.

## Workflow

1. Read root `AGENTS.md` and any closer local `AGENTS.md`.
2. Read `docs/engineering/baseline.md` before choosing a framework, library, or implementation shape.
3. If the task touches an external boundary, also read the relevant files in `contracts/`, `fixtures/`, and `examples/`.
4. Search the repo for prior art before designing a new helper, abstraction, wrapper, or dependency.
5. Choose the lowest reuse tier that solves the task safely.
6. Keep custom code thin and explicitly limited to orchestration, adapters, contract projection, data transformation, and repo-specific business rules.
7. In your summary, name the reused building blocks and call out any unavoidable original glue.

## Reuse Ladder

Choose options in this order:

1. Existing repo code and the frozen stack
2. Standard library or built-in platform capability
3. Official SDK or upstream dependency already frozen in the repo
4. Mature, production-validated OSS with the smallest practical dependency surface
5. Thin custom glue

Do not skip to a lower tier until the higher tier is demonstrably insufficient.

## Reuse Anchors

- Server: start from `server/internal/*`, especially existing repositories, services, HTTP handlers, runtime, adapter, scheduler, storage, and logging packages.
- Web: start from `web/src/lib/http.ts`, `web/src/lib/ws.ts`, `web/src/stores/*`, `web/src/components/*`, and existing page patterns.
- Launcher: start from `launcher/src/main/services/*`, `launcher/src/shared/*`, and the current Electron `main` / `preload` / `renderer` split.
- Contracts and examples: use `contracts/`, `fixtures/`, and `examples/` as the first stop for frozen shapes, sample payloads, and regression anchors.

## Dependency Gate

Before introducing a new dependency, explicitly check all of the following:

- The repo does not already contain a suitable implementation or frozen stack choice.
- The standard library or platform capability is not enough.
- The candidate is official or well maintained.
- The license is clear and acceptable for the repo.
- The project shows real production adoption or strong maintenance signals.
- The added surface area is narrow and does not duplicate a stack already frozen in repo docs.
- The version can be pinned through the existing lockfile, manifest, or engineering file for that surface.

If any item fails, prefer the higher reuse tier or write the smallest possible glue code instead.

## Do Not

- Do not introduce a second router, ORM, logging stack, state manager, HTTP client, WebSocket client, UI component system, or launcher service layer without proving the frozen choice is insufficient.
- Do not fork, vendor, or patch upstream libraries when a black-box integration works.
- Do not create generic future-proof abstractions when a direct integration with current repo patterns is enough.
- Do not describe fresh handwritten code as glue if it is actually reimplementing a mature wheel.
