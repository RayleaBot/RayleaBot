# RayleaBot Repository Guide

## 项目目标与 v0.1 边界摘要

RayleaBot 是一个面向个人开发者和 GitHub 开源协作者的自托管聊天机器人框架。
v0.1 的闭环目标是：

- QQ / OneBot11 反向 WebSocket 单协议接入。
- 单实例运行与基础生命周期管理。
- Python / Node.js 两类插件运行能力。
- Web 管理面板。
- 平台级图片渲染服务。
- SQLite 状态与管理数据存储。
- Windows 专属 Launcher。

v0.1 暂不纳入：

- 多协议并行接入。
- 正向 WebSocket、HTTP 上报、HTTP API 组合等其他 OneBot11 传输模式。
- 分布式、多节点、高可用。
- 正式插件市场。
- 强沙盒与完整资源配额控制。
- Rust / Go 官方运行时支持。

## 单向优先级

本仓库的单向优先级固定为：

`规划文档 > contracts/ > schema/fixtures > code`

补充规则：

- `contracts/` 是对外接口、schema、错误码的唯一正式来源。
- Markdown 用于解释，不是最终接口裁决依据。
- 若 Markdown 与 `contracts/` 冲突，以 `contracts/` 为准，并在同一变更中修正文档说明。
- 若某项契约本轮无法完全写完，也必须先产出正式骨架文件和 `TODO`，不能直接跳去写实现。

## 目录职责

| 路径 | 职责 |
| --- | --- |
| `contracts/` | 对外 HTTP API、WebSocket、plugin manifest、plugin protocol、config schema、error codes、release manifest 的正式来源 |
| `docs/engineering/` | baseline、CI 门禁、实施顺序、仓库治理 |
| `docs/architecture/` | 架构设计、状态模型、统一事件模型、跨层边界 |
| `docs/dev/` | 本地开发、调试、诊断、贡献流程 |
| `docs/plugin/` | 插件开发文档、manifest、Capabilities、协议与生命周期 |
| `docs/plugin/sdk/` | Python / Node.js 官方 SDK 文档 |
| `docs/user/` | 安装、初始化、配置、运行、恢复与排障 |
| `docs/release/` | 版本说明、迁移说明、已知问题 |
| `fixtures/` | Golden cases、契约样例、回归基线 |
| `examples/` | 示例插件、示例配置、示例请求/响应 |
| `server/` | Go 服务端工程；Phase 0 只允许 baseline，不允许写业务实现 |
| `web/` | Web UI 工程；Phase 0 只允许 baseline，不允许写业务实现 |
| `launcher/` | Windows Launcher 工程；Phase 0 只允许 baseline，不允许写业务实现 |
| `.deps/` | Chromium、托管 Python / Node.js 运行时及相关资源清单 |
| `config/` | 默认配置模板与用户配置入口 |
| `data/` | SQLite 状态库与运行数据 |
| `cache/` | 渲染缓存、下载缓存、插件临时缓存 |
| `logs/` | 结构化日志与诊断输出 |
| `references/` | 参考资料，不是正式契约来源 |

## 默认构建命令与测试命令

### Server

- 构建：`go build ./cmd/raylea-server`
- 测试：`go test ./...`

### Web

- 安装：`pnpm install --frozen-lockfile`
- 开发：`pnpm dev`
- 构建：`pnpm build`
- 单元测试：`pnpm test`
- E2E：`pnpm test:e2e`

### Launcher

- 构建：`dotnet publish ./launcher -c Release`
- 测试：`dotnet test ./launcher`

规则：

- Server、Web、Launcher 各自只允许一种默认构建入口。
- CI 必须直接复用这些默认命令，不允许本地、CI、发布各维护一套不一致入口。

## 变更四件套门禁

任一涉及以下边界的变更：

- 协议
- schema
- 状态机
- 配置
- 数据库结构
- 插件安装流程
- 渲染输入输出
- Web API
- WebSocket
- 错误码
- 迁移

合并前必须同时满足四件套：

- 实现代码更新
- 契约更新
- 测试更新
- 示例更新

禁止“代码先改，契约/测试/示例以后补”。

## 禁止跨层补洞清单

禁止以下捷径：

- Adapter 直接写状态库。
- Launcher 复制 Web 业务逻辑。
- Web UI 通过解析日志推断真实状态。
- 插件运行时直接读写 `config/user.yaml`。
- 插件绕过 Capability 校验直连平台内部模块。
- CLI、Launcher、Web UI 各自发明不同状态名、错误码或任务状态。
- 在 `contracts/` 之外定义新的对外接口字段并直接实现。
- 在未更新 `contracts/` 的情况下引入新的状态名、新错误码、新目录职责、新接口。

## Phase 0 强制起步规则

本项目第一次实现必须从 `contracts/` 和 `baseline` 开始，不能先写功能代码。

Phase 0 只做：

- 仓库骨架。
- 文档拆分骨架。
- baseline 文件。
- contracts 骨架。
- fixtures / examples 骨架。
- 最小 CI 骨架。

Phase 0 不做：

- Adapter 正式实现。
- Runtime / Runtime Bridge 正式实现。
- Web API 正式实现。
- Web UI 正式实现。
- Launcher 正式实现。
- Render Service 正式实现。
