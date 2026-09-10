# Managed Runtime Dependency Boundary

`internal/deps` 管理 `.deps/manifest.json` v5 声明的 Chromium 与 FFmpeg。Chromium 资源声明 `entrypoints.browser`，FFmpeg 资源同时声明 `entrypoints.ffmpeg` 与 `entrypoints.ffprobe`；插件后端仍是预编译 Go artifact，不从这里请求 Python、Node.js 或 npm。

## Responsibilities

- 加载 manifest，选择当前平台的 Chromium 与 FFmpeg 资源，并按资源种类校验可信来源、归档格式、SHA-256 和必需 entrypoint 元数据。
- 按 source 策略下载并校验归档；下载缓存位于 `cache/downloads/runtime/`，准备锁位于 `cache/downloads/platform.lock`。
- 把受验证归档展开到 `.deps/store/<id>/<version>/`，解析对应入口；Chromium 在允许时可使用系统 Chrome、Chromium 或 Edge，FFmpeg 使用清单固定的托管入口。
- 通过 `BootstrapInspection`、`PrepareReport`、`PrepareProgress` 和 `BootstrapError` 提供只读诊断、准备结果、进度与结构化失败信息。

## Public Boundaries

- 可能准备资源或解析入口的调用方使用 `NewRuntime(repoRoot)`。
- 只读诊断使用 `NewDiagnostics(repoRoot)` 与 `InspectRuntime("chromium"|"ffmpeg")`。
- 仅检查清单时使用 `LoadManifest`、`CurrentPlatform` 和元数据 helper。
- source 选择、下载、校验、解压、缓存布局和锁处理留在 `internal/deps` 内。

## Caller Rules

- 图片渲染和抖音扫码登录（浏览器兜底）通过 runtime boundary 请求 Chromium `browser` 入口，不直接遍历缓存或 `.deps/store`。
- 启动准备同时覆盖当前平台的 Chromium 与 FFmpeg；插件 runtime 只读取已准备的 `ffmpeg` / `ffprobe` 入口，并通过 `RAYLEABOT_FFMPEG_PATH`、`RAYLEABOT_FFPROBE_PATH` 注入受信本地插件进程，不在插件启动时下载资源。
- CLI doctor 与系统诊断只使用 diagnostics boundary，不在只读检查中准备资源。
- 插件安装不依赖 `internal/deps`；插件 runtime 只运行已校验的 Go artifact，并可消费核心提供的媒体工具入口。
- 用户可见的准备失败应保留 `BootstrapError` 的 stage、source、路径和 remediation，并用现有摘要 helper 生成一致文案。

## 归档与下载边界

托管资源使用流式 HTTPS 下载，最多 2 GiB，保留取消、总超时、空闲超时和 SHA-256 校验；临时文件在失败后回收。归档逐项在 os.Root 内展开，最多 100,000 条目、单文件 2 GiB、累计 8 GiB；拒绝越界、重复路径和特殊文件。macOS framework 所需的内部相对链接在普通文件之后创建，再校验最终解析范围。

XZ 使用纯 Go `github.com/xi2/xz` 固定版本 `v0.0.0-20171230120015-48954b6210f8`，替代外部 tar，以执行逐条路径检查、取消和 64 MiB 字典上限。第三方声明保留上游 LICENSE 的 public-domain 声明。插件包和发行更新的更严格策略仍由各自服务执行。

## 清单验证

Server、Launcher 与发布 Python 工具共享 deps-manifest schema 和正反 fixtures。全部来源/入口候选必须有效；拒绝重复资源 ID、平台与种类组合和同资源重复来源 URL。Launcher 使用已有 Server 技术栈的 jsonschema/v6 执行正式 schema，不再维护另一份字段校验规则。
