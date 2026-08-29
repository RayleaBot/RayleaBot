# Chromium Dependency Boundary

`internal/deps` 只管理 `.deps/manifest.json` v4 声明的 Chromium。manifest 只接受 `kind: "chromium"`，且每项只能声明 `entrypoints.browser`；插件后端是预编译 Go artifact，不从这里请求 Python、Node.js 或 npm。

## Responsibilities

- 加载 manifest，选择当前平台的 Chromium 资源并校验多源、归档格式、SHA-256 和 browser entrypoint 元数据。
- 按 source 策略下载并校验归档；下载缓存位于 `cache/downloads/runtime/`，准备锁位于 `cache/downloads/platform.lock`。
- 把受验证归档展开到 `.deps/store/<id>/<version>/`，解析 `browser` 入口，并在允许时使用系统 Chrome、Chromium 或 Edge。
- 通过 `BootstrapInspection`、`PrepareReport`、`PrepareProgress` 和 `BootstrapError` 提供只读诊断、准备结果、进度与结构化失败信息。

## Public Boundaries

- 可能准备资源或解析入口的调用方使用 `NewRuntime(repoRoot)`。
- 只读诊断使用 `NewDiagnostics(repoRoot)` 与 `InspectRuntime("chromium")`。
- 仅检查清单时使用 `LoadManifest`、`CurrentPlatform` 和元数据 helper。
- source 选择、下载、校验、解压、缓存布局和锁处理留在 `internal/deps` 内。

## Caller Rules

- 图片渲染和抖音扫码登录（浏览器兜底）通过 runtime boundary 请求 Chromium `browser` 入口，不直接遍历缓存或 `.deps/store`。
- CLI doctor 与系统诊断只使用 diagnostics boundary，不在只读检查中准备资源。
- 插件安装与插件 runtime 不依赖 `internal/deps`；它们只运行已校验的 Go artifact。
- 用户可见的准备失败应保留 `BootstrapError` 的 stage、source、路径和 remediation，并用现有摘要 helper 生成一致文案。
