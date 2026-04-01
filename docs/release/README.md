# Release Docs

本目录用于承载 RayleaBot 的版本说明、迁移说明、已知问题和交付约束。

## 当前发布边界

- `contracts/release-manifest.schema.json` 已定义正式 release metadata 结构，并进入 fixture-ready。
- 当前仓库的真实交付面覆盖 server、web、launcher、contracts、fixtures、examples、builtin 插件、`templates/` 与 `.deps/manifest.json`。
- `.deps/manifest.json` 已固定 Chromium 资源的正式版本、来源、SHA256 与平台矩阵；Python / Node.js runtime metadata 仍保留后续补齐空间。

## 文档关注点

- 记录当前可验证的版本内容、迁移影响和已知限制。
- 说明 release metadata、资源清单与打包目录布局的正式字段来源。
- 说明 release workflow、smoke 校验与产物矩阵的当前交付约束。

## 维护规则

- 发行元数据以 `contracts/release-manifest.schema.json` 为准。
- 本目录用于说明发布内容，不裁决 manifest 字段、签名结构或资源清单字段。
- 若发布流程、产物矩阵或 manifest 结构变化，先更新正式契约，再同步本目录说明。
