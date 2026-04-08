# Delivery and Upgrade

本页说明 RayleaBot 当前正式发行物、发布元数据、包目录真相和升级回滚边界。

## 构建与交付目标

- 一键运行。
- 降低首次部署复杂度。
- 尽量避免用户手动安装 Python / Node.js 运行环境。
- 把 Chromium 渲染环境收口到平台受控资源，而不是交给插件各自维护。

## 正式产物矩阵

| `artifact_id` | 产物形态 | `support_level` | `smoke_profile` | 说明 |
| --- | --- | --- | --- | --- |
| `windows-x64-full` | 桌面完整包 | `First-class` | `windows_full_smoke` | 含 `raylea-server.exe`、Launcher、Web、内置插件和 `.deps/manifest.json` |
| `linux-x64-full` | 桌面完整包 | `First-class` | `linux_full_smoke` | 含 `raylea-server`、Launcher、Web、内置插件和 `.deps/manifest.json` |
| `macos-arm64-full` | 桌面完整包 | `First-class` | `macos_full_smoke` | 含 `raylea-server`、Launcher `.app`、Web、内置插件和 `.deps/manifest.json` |
| `linux-x64-server` | 服务端包 | `First-class` | `linux_server_smoke` | 含 `raylea-server`、Web、内置插件、运行环境资源与 `systemd` 示例 |

## 发布包目录

正式包根目录按产物形态包含以下正式内容：

- `raylea-server`
- Launcher 或等价桌面入口
- `web/dist`
- `build_info.json`
- `contracts/`
- `config/default.yaml`
- `plugins/builtin/`
- `templates/`
- `.deps/manifest.json`

发行包根目录同时是默认运行根目录，安装、升级和工作区复用都以这一结构为准。

## 发布元数据

每次正式 Release 同时发布：

- `release_manifest.json`
- `build_info.json`
- `SHA256SUMS.txt`

元数据用于校验：

- 版本号与提交哈希
- 产物标识与平台矩阵
- 配置 schema、数据库 schema 和插件协议版本
- 产物摘要与大小
- 对应 `.deps/manifest.json` 摘要

### `release_manifest.json`

| 字段 | 作用 |
| --- | --- |
| `version` | RayleaBot 版本号 |
| `git_commit` | 对应提交哈希 |
| `built_at` | 构建时间 |
| `config_schema_version` | 配置 schema 版本 |
| `db_schema_version` | 数据库 schema 版本 |
| `plugin_protocol_version` | 插件协议版本 |
| `artifacts` | 产物列表，含 `artifact_id`、文件名、平台、摘要、大小、`support_level`、`deps_manifest_sha256` 与 `smoke_profile` |
| `release_notes_ref` | 对应版本说明或 Release 地址 |

### `build_info.json`

| 字段 | 作用 |
| --- | --- |
| `version` | 当前包对应的 RayleaBot 版本 |
| `git_commit` | 构建提交哈希 |
| `artifact_id` | 当前包的产物标识 |
| `built_at` | 构建时间 |
| `release_manifest_sha256` | 对应 `release_manifest.json` 的摘要 |

## 升级与回滚

- 升级默认不覆盖 `config/`、`data/` 和 `plugins/installed/`。
- 升级前先检查配置版本、数据库版本和插件兼容风险。
- 回滚依赖升级前备份，不直接让旧版本读取较新的状态库。
- 恢复后先执行迁移和兼容检查，再决定是否进入可运行状态。
- 跨平台恢复默认只保证配置和业务数据可参考恢复，不保证二进制插件与运行环境直接复用。

## 恢复支持矩阵

| 场景 | 支持级别 | 说明 |
| --- | --- | --- |
| 同平台、同小版本线恢复 | 支持 | v0.1 的主要受控恢复路径 |
| 同平台、跨小版本恢复 | 受控支持 | 需先通过迁移与兼容检查 |
| 跨大版本恢复 | 默认不支持 | 需要额外迁移说明或显式拒绝 |
| 跨平台恢复 | 仅配置与业务数据参考恢复 | `.deps/`、运行环境与二进制插件不保证可直接复用 |

## 当前边界

- GitHub 自动生成的源代码压缩包不属于正式运行时交付产物。
- 自动覆盖更新不在当前正式范围。

## 相关文档

- [Acceptance and Risks](./acceptance-and-risks.md)
- [Deployment](../user/deployment.md)
