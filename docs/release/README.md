# Release Docs

本目录用于承载 RayleaBot 的版本说明、迁移说明、已知问题和交付约束。

## 当前发布边界

- `contracts/release-manifest.schema.json` 已定义正式 release metadata 结构，并进入 fixture-ready。
- 正式发行包当前包含 server、Web 管理面静态资源、Launcher（full artifact）、builtin 插件、`contracts/`、`templates/`、`.deps/manifest.json`、`config/default.yaml` 与 release metadata sidecar。
- `fixtures/`、`examples/`、开发脚本与仓库治理文件属于仓库内容，不属于正式发行包交付面。
- `.deps/manifest.json` 已固定 Chromium、Python 与 Node.js 资源的正式版本、来源、SHA256、archive_format、entrypoints 与平台矩阵；Python `3.12.13` 记录 `python-build-standalone` 便携发行物来源，Node.js `24.14.0` 记录正式平台归档来源。

## 正式包目录真相

- `windows-x64-full` 的正式桌面入口是包根目录 `RayleaLauncher.exe`。
- `linux-x64-full` 的正式桌面入口是包根目录 `RayleaLauncher`。
- `macos-arm64-full` 的正式桌面入口是包根目录 `RayleaLauncher.app`。
- `linux-x64-server` 的正式入口是包根目录 `raylea-server`，同时附带 `systemd/rayleabot.service` 示例文件。
- full artifact 根目录统一包含 `raylea-server`、Launcher、`web/dist`、`contracts/`、`config/default.yaml`、`plugins/builtin/`、`templates/` 与 `.deps/manifest.json`。
- 发行包根目录同时是默认运行根目录；安装、升级与工作区复用说明都以这一目录结构为准。

## 文档关注点

- 记录当前可验证的版本内容、迁移影响和已知限制。
- 说明 release metadata、资源清单与打包目录布局的正式字段来源。
- 说明 release workflow、smoke 校验、跨版本 recovery drill、长期自托管 smoke、恢复摘要与产物矩阵的当前交付约束。

## 当前门禁

- `lint.yml` 的 `smoke-pr` 会对 `linux-x64-full` 与 `linux-x64-server` 正式包执行打包、smoke、跨版本 recovery drill 与 metadata verify。
- `lint.yml` 的 `ci-launcher` 会在 Windows 与 macOS runner 上补齐对应 full artifact 的打包、smoke 与跨版本 recovery drill。
- `release.yml` 会对 `windows-x64-full`、`linux-x64-full`、`linux-x64-server`、`macos-arm64-full` 依次执行打包、smoke、跨版本 recovery drill、长期自托管 smoke、release metadata 校验与发布。
- `self-host-smoke.yml` 提供 `workflow_dispatch` 手动回归入口，可按 artifact 子集复用正式打包路径并执行同一套长期自托管 smoke 脚本。
- full artifact 的 smoke 与 recovery drill 统一校验根目录 Launcher 入口，不依赖 `launcher/` 子目录布局。
- 跨版本 recovery drill 使用已发布 release asset、`release_manifest.json` 与发行包内 `build_info.json` 作为版本来源；找不到前一正式版时输出显式 bootstrap skip。
- release smoke 会校验正式包内 `.deps/manifest.json` 的 runtime bootstrap 前置条件；recovery drill 与长期自托管 smoke 会在启动前按需准备受控运行时。
- 长期自托管 smoke 直接启动发行包内 `raylea-server`，覆盖初始化、登录、在线诊断导出、在线备份、重启后再探活的长时间窗巡检；默认窗口为 600 秒，默认探针间隔为 30 秒。
- 跨版本 recovery drill 会在 PR 级门禁使用 60 秒观察窗口，在 tag release 与手动回归使用 300 秒观察窗口，覆盖兼容通过与需要人工处理两类恢复场景，并持续比对 API、本地 `logs/recovery-summary.json` 与 diagnostics 中的恢复摘要。
- `restore` 预检与启动后兼容检查会把共享恢复摘要写入 `logs/recovery-summary.json`；CLI、Web、Launcher 与 diagnostics 都复用同一份摘要。
- `post_startup` 恢复摘要在 `degraded` 状态下会稳定保留 `manual_actions`、`next_steps` 与跳过插件列表，在 `compatible` 状态下不保留人工处理建议。
- 正式包只携带 `.deps/manifest.json`，受控 Chromium、Python 与 Node.js 运行时按需下载到 `cache/downloads/runtime/`，并展开到 `.deps/store/<resource-id>/<version>/`；有效运行时根目录继续跟随发行包根目录或 `config/user.yaml` 所在根目录，不随 Launcher `workdir` 覆盖改变。

## 维护规则

- 发行元数据以 `contracts/release-manifest.schema.json` 为准。
- 本目录用于说明发布内容，不裁决 manifest 字段、签名结构或资源清单字段。
- 若发布流程、产物矩阵或 manifest 结构变化，先更新正式契约，再同步本目录说明。
