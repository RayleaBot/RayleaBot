# RayleaBot Engineering Baseline

## 目的

本文件固定 RayleaBot 的工程版本线、默认命令、目录职责和长期有效的实现选型。
进入正式实现后，AI 与人工协作都应先读本文件，再读 `contracts/`，最后才进入代码与文档修改。

单向优先级：

`docs/RayleaBot机器人项目规划.md > contracts/ > fixtures/examples > code`

补充说明：

- 对外接口、schema、错误码最终以 `contracts/` 为准。
- 工程工具链、默认命令、目录职责最终以本文件和对应工程文件为准。
- 若规划文档与工程基线冲突，先在本文件和对应工程文件中收敛，再同步相关说明。

## 当前工程落点

- `server/` 是产品核心，承载配置、存储、鉴权、任务、插件发现、OneBot11 adapter、多插件 runtime、dispatcher、scheduler trigger 与管理面日志持久化。
- `web/` 承载管理控制台主链路。
- `launcher/` 承载 Electron 桌面启动器，负责本地环境检查、服务进程编排、桌面交互与打开 Web 管理面。
- `.deps/manifest.json` 固定 Chromium 与 Python / Node.js 运行环境资源矩阵及其有序来源列表，并作为运行环境准备的唯一正式来源。
- 运行环境有效根目录按 `config/user.yaml` 的上两级目录推导；Launcher `workdir` 只承担进程工作目录与日志目录职责，不覆盖 `.deps/` 与 `templates/` 的位置。
- 恢复人工处理与运行环境准备继续复用共享任务模型；`recovery.recheck` 与 `runtime.bootstrap` 是当前正式操作入口。

## v0.1 固定版本线

| 领域 | 固定基线 |
| --- | --- |
| Server | Go `1.25.8` |
| Web / Node runtime | Node.js `24.14.0` |
| JS package manager | `pnpm 10.32.1` |
| Web UI | Vue `3.5.30` + Vite `8.0.0` + Element Plus `2.13.5` + Vue Router `5.0.3` + Pinia `3.0.4` |
| Launcher runtime | Electron `41.1.0` + TypeScript `6.0.2` + React `18.3.1` + Fluent UI React v9 + Vite `8.0.3` + `@vitejs/plugin-react 6.0.1` + `electron-builder 26.8.1` |
| Python runtime | Python `3.12.13` |
| Database | SQLite via `modernc.org/sqlite v1.47.0` |
| Render | `chromedp 0.14.2` + Chromium 浏览环境 |

## 固定工程选型

| 领域 | 固定选型 |
| --- | --- |
| HTTP 路由 | `net/http` + `go-chi/chi v5.2.5` |
| WebSocket | `github.com/coder/websocket v1.8.14` |
| 日志 | `log/slog` |
| 配置解析 | `gopkg.in/yaml.v3` |
| 数据访问 | `database/sql` + repository / service 分层 + 手写 SQL |
| Web 路由 | Vue Router `5.x` |
| Web 全局状态 | Pinia `3.x` |
| Web HTTP | 原生 `fetch` + 薄封装 |
| Web 实时通信 | 原生 `WebSocket` + 薄封装 |
| Web 样式 | Element Plus + Vue SFC `lang="scss"` + CSS Variables |
| Launcher 主进程 | Electron `main` + typed service layer |
| Launcher 桌面桥接 | `preload` 暴露受限 IPC API |
| Launcher 渲染层 | React 18 + Fluent UI React v9 + Vite 单页面桌面壳 |
| 仓库级 JS 包管理器 | `pnpm` |
| Node.js 插件依赖安装器 | `npm` |
| Python 插件依赖安装链路 | Python 运行环境 + 每插件独立 `.venv/` |
| Node.js 插件运行链路 | Runtime 注入 `--max-old-space-size=<limit_mb>`，默认 `256 MB` |
| 运行环境资源准备 | `.deps/manifest.json` + `cache/downloads/runtime/` + `.deps/store/<resource-id>/<version>/` |

## 默认命令

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

- 安装：`pnpm install --frozen-lockfile`
- 测试：`pnpm test`
- 构建：`pnpm build`

## 目录职责

| 路径 | 职责 |
| --- | --- |
| `contracts/` | 对外正式契约根目录 |
| `docs/engineering/` | 工程基线、CI、实施顺序、治理规则 |
| `docs/architecture/` | 架构、状态模型、事件模型、边界说明 |
| `docs/dev/` | 开发、调试、诊断、贡献流程 |
| `docs/plugin/` | 插件 manifest、Capabilities、协议、生命周期 |
| `docs/plugin/sdk/` | Python / Node.js SDK 说明 |
| `docs/user/` | 用户安装、初始化、配置、运行、恢复 |
| `docs/release/` | 版本说明、迁移说明、已知问题 |
| `fixtures/` | Golden fixtures 与可执行样例 |
| `examples/` | 示例插件、示例配置、示例请求/响应 |
| `server/` | Go 服务端工程 |
| `web/` | Web UI 工程 |
| `launcher/` | Electron 桌面启动器工程 |
| `.deps/` | Chromium 与 Python / Node.js 运行环境资源清单，以及按需展开后的运行环境目录 |
| `config/` | 默认配置模板与用户配置 |
| `data/` | SQLite 状态库与运行数据 |
| `cache/` | 渲染缓存、下载缓存、插件临时缓存 |
| `logs/` | 结构化日志与诊断输出 |

## 仓库级强制基线文件

| 路径 | 约束 |
| --- | --- |
| `server/go.mod` | 固定 `module rayleabot/server`、Go `1.25.8` 与 server 依赖版本 |
| `server/go.sum` | 维护 server 依赖锁定结果 |
| `web/package.json` | 固定 `packageManager = pnpm@10.32.1` 与 `engines.node = 24.14.0` |
| `web/pnpm-lock.yaml` | 作为 Web 工程唯一 JS 锁文件 |
| `launcher/package.json` | 固定 `packageManager = pnpm@10.32.1`、`engines.node = 24.14.0`、Electron/Vite/React/`@vitejs/plugin-react`/build 脚本与打包配置 |
| `launcher/pnpm-lock.yaml` | 作为 Launcher 工程唯一 JS 锁文件 |
| `.deps/manifest.json` | 固定资源名、版本线、有序来源列表、SHA256、archive_format、entrypoints 与平台矩阵 |
| `contracts/*` | 对外接口与错误码唯一正式来源 |

## 已冻结的规范化决议

- `contracts/config.user.schema.json` 中 `server.host` 默认值采用 `127.0.0.1`。
- OneBot 连接地址正式键名采用 `onebot.ws_url`。
- `launcher/package.json` 锁定 Electron 启动器的脚本入口、打包形态与 Node / pnpm 基线。
- `server/go.mod` 当前采用 `rayleabot/server` 作为 module path。

## `contracts/` 作为正式来源

以下边界的最终裁决不在 Markdown，而在 `contracts/`：

- 插件 manifest：`contracts/plugin-info.schema.json`
- 插件 JSONL 协议：`contracts/plugin-protocol.schema.json`
- HTTP API：`contracts/web-api.openapi.yaml`
- WebSocket：`contracts/websocket-events.yaml`
- 用户配置：`contracts/config.user.schema.json`
- 错误码：`contracts/error-codes.yaml`
- 发行元数据：`contracts/release-manifest.schema.json`
- CLI：`contracts/cli-commands.yaml`

## 当前仍需保留的基线 TODO

- `TODO(repo.identity)`：仓库配置正式 remote 后，将 `server/go.mod` 的本地 module path 收敛为正式模块路径。

规则：

- 上述 TODO 进入真实运行链路前，需要先在 baseline、相关工程文件和契约说明中一并收敛。
- 若后续变更尝试绕开 baseline 与 contracts 直接写功能代码，应视为违反仓库治理规则。
