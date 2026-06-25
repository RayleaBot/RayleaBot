# Dependency Runtime Boundaries

`internal/deps` owns managed runtime resources declared in `.deps/manifest.json`. It covers Chromium, Python, Node.js, and npm runtime preparation for local development, packaged releases, CI images, and self-hosted deployments.

## Responsibilities

- Manifest metadata: load `.deps/manifest.json`, select the current platform resource, and verify required metadata.
- Download and verification: choose download sources, fetch archives, verify checksums, and keep cache paths under `.deps/cache`.
- Runtime preparation: unpack verified archives into `.deps/store`, resolve declared entrypoints, and fall back to system Chromium when allowed.
- Diagnostics: report whether a runtime is ready, cached, on-demand, missing, or misconfigured, including user-readable remediation.

## Public Boundaries

- Runtime callers use `NewRuntime(repoRoot)` for operations that may prepare or resolve runtime entrypoints.
- Diagnostic callers use `NewDiagnostics(repoRoot)` for read-only runtime status checks.
- Manifest-only checks use `LoadManifest`, `CurrentPlatform`, and metadata helpers.
- Download, archive extraction, cache layout, and lock handling stay inside `internal/deps`.

## Caller Rules

- Render code may ask for the Chromium `browser` entrypoint through the runtime boundary. It must not choose download sources, verify archives, unpack files, or inspect `.deps/store` directly.
- CLI doctor and system diagnostics may inspect dependency status through the diagnostics boundary. They must not prepare dependencies as part of read-only checks.
- Plugin install and plugin runtime code may request Python, Node.js, or npm entrypoints through the runtime boundary. They must not depend on archive format or cache paths.
- User-facing errors should use `BootstrapError`, `BootstrapSummary`, and `BootstrapRemediation` so offline and missing-runtime cases give a concrete repair path.
