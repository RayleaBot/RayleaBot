# Release Docs

本目录用于承载 RayleaBot 的版本说明、迁移说明、已知问题和交付约束。

## 当前发布边界

- `contracts/release-manifest.schema.json` 已定义正式 release metadata 结构，并进入 fixture-ready。
- 当前仓库的真实交付面仍以 server、contracts、fixtures、examples 与 builtin 资源为主。
- `.deps/manifest.json` 仍是受控 Chromium 与托管运行时资源清单骨架，来源、SHA256 与 Chromium 正式版本尚待补齐。

## 文档关注点

- 记录当前可验证的版本内容、迁移影响和已知限制。
- 说明 release metadata 与资源清单的正式字段来源。
- 在 Web UI、Launcher、Render Service 进入实现前，本目录不预设完整安装器、升级服务或交付矩阵。

## 维护规则

- 发行元数据以 `contracts/release-manifest.schema.json` 为准。
- 本目录用于说明发布内容，不裁决 manifest 字段、签名结构或资源清单字段。
- 若发布流程、产物矩阵或 manifest 结构变化，先更新正式契约，再同步本目录说明。
