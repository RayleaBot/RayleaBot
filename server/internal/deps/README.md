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

## Input and Archive Limits

Server、Launcher 和发布脚本在使用清单前校验同一份 `contracts/deps-manifest.schema.json`，并运行 `fixtures/deps-manifest/` 中的共享正反例。所有来源与入口候选都必须有效，资源 ID、平台/种类组合及同资源来源 URL 均不得重复。

托管资源下载最多 2 GiB，仅接受 HTTPS，禁止携带 URL 凭据及降级重定向。流式下载保留取消、总超时和空闲超时，失败删除临时文件；正式缓存只在 SHA-256 校验成功后发布。发布更新继续使用签名清单中的精确大小、可信下载地址及磁盘预检策略。

ZIP、tar.gz、tar.xz 在私有临时目录内逐项展开：最多 100,000 条目、单文件 2 GiB、累计 8 GiB；拒绝越界、重复路径和特殊设备文件，保留执行位。macOS Chromium framework 所需的归档内相对链接在普通文件写入结束后建立，并再次验证完整解析仍位于根内。插件包和正式更新保留各自更严格的文件数、大小、压缩比和签名策略。

XZ 使用纯 Go `github.com/xi2/xz` 固定版本 `v0.0.0-20171230120015-48954b6210f8`，替代外部 `tar`，使取消、逐条路径检查和字典上限可执行。解码字典最多 64 MiB；发布 Python 解码器另为解码状态保留 2 MiB。第三方声明收录上游 LICENSE 的 public-domain 声明。
