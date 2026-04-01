# Release Docs

本目录用于承载 RayleaBot 的版本说明、迁移说明、已知问题和交付约束。

## 当前发布边界

- `contracts/release-manifest.schema.json` 已定义正式 release metadata 结构，并进入 fixture-ready。
- 正式发行包当前包含 server、Web 管理面静态资源、Launcher（full artifact）、builtin 插件、`contracts/`、`templates/`、`.deps/manifest.json`、`config/default.yaml` 与 release metadata sidecar。
- `fixtures/`、`examples/`、开发脚本与仓库治理文件属于仓库内容，不属于正式发行包交付面。
- `.deps/manifest.json` 已固定 Chromium 资源的正式版本、来源、SHA256 与平台矩阵；Python / Node.js runtime metadata 仍保留后续补齐空间。

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
- 说明 release workflow、smoke 校验、packaged recovery drill 与产物矩阵的当前交付约束。

## 当前门禁

- `lint.yml` 的 `smoke-pr` 会对 `linux-x64-server` 正式包执行 packaged recovery drill，覆盖 `backup` / `restore` / `doctor` 与恢复后最小启动探活。
- `release.yml` 会对 `windows-x64-full`、`linux-x64-full`、`linux-x64-server`、`macos-arm64-full` 依次执行打包、smoke、packaged recovery drill、release metadata 校验与发布。
- full artifact 的 smoke 与 packaged recovery drill 统一校验根目录 Launcher 入口，不再依赖 `launcher/` 子目录布局。
- packaged recovery drill 的验证边界是同版本正式包的受控恢复闭环，不表达跨版本 upgrade / rollback 演练。

## 维护规则

- 发行元数据以 `contracts/release-manifest.schema.json` 为准。
- 本目录用于说明发布内容，不裁决 manifest 字段、签名结构或资源清单字段。
- 若发布流程、产物矩阵或 manifest 结构变化，先更新正式契约，再同步本目录说明。
