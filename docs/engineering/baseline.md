# RayleaBot Engineering Baseline

## 目的

本文件是 RayleaBot v0.1 的工程基线说明。
进入正式实现后，AI 与人工协作都必须先读本文件，再读 `contracts/`，最后才允许写代码。

单向优先级：

`docs/RayleaBot机器人项目规划.md > contracts/ > fixtures/ > code`

补充说明：

- 对外接口、schema、错误码最终以 `contracts/` 为准。
- 工程工具链、默认命令、目录职责最终以本文件和对应工程文件为准。
- 若规划文档与工程基线冲突，先在本文件和对应工程文件中收敛，再更新规划文档说明。

## v0.1 固定版本线

| 领域 | 固定基线 |
| --- | --- |
| Server | Go `1.25.8` |
| Web / Node runtime | Node.js `24.14.0` |
| JS package manager | `pnpm 10.32.1` |
| Web UI | Vue `3.5.30` + Vite `8.0.0` + Element Plus `2.13.5` + Vue Router `5.0.3` + Pinia `3.0.4` |
| Python runtime | Python `3.12.13` |
| Database | SQLite via `modernc.org/sqlite` |
| Launcher | `.NET 10` LTS line，Phase 0 锁定 SDK `10.0.103` |
| Avalonia | `11.3.12` |
| Render | `chromedp 0.13.2` + 受控 Chromium |

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
| 仓库级 JS 包管理器 | `pnpm` |
| Node.js 插件依赖安装器 | `npm` |
| Python 插件依赖安装链路 | 受控 Python + 每插件独立 `.venv/` |
| Node.js 插件运行链路 | Runtime 注入 `--max-old-space-size=<limit_mb>`，默认 `256 MB` |

## 默认命令

### Server

- 构建：`go build ./cmd/raylea`
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
| `launcher/` | Windows Launcher 工程 |
| `.deps/` | Chromium 与托管运行时资源清单 |
| `config/` | 默认配置模板与用户配置 |
| `data/` | SQLite 状态库与运行数据 |
| `cache/` | 渲染缓存、下载缓存、插件临时缓存 |
| `logs/` | 结构化日志与诊断输出 |

## 仓库级强制基线文件

| 路径 | 约束 |
| --- | --- |
| `server/go.mod` | 必须锁定 Go `1.25.8` |
| `server/go.sum` | Phase 0 可为空，占位即可 |
| `web/package.json` | 必须锁定 `packageManager = pnpm@10.32.1` 与 `engines.node = 24.14.0` |
| `web/pnpm-lock.yaml` | 作为唯一 JS 锁文件 |
| `launcher/global.json` | 固定 `.NET SDK 10.0.103` 与 `latestPatch` |
| `launcher/Directory.Packages.props` | 集中锁定 Avalonia `11.3.12` |
| `.deps/manifest.json` | 固定资源名、版本线、来源、SHA256 与平台矩阵 |
| `contracts/*` | 对外接口与错误码唯一正式来源 |

## Phase 0 规范化决议

以下决议用于收敛规划文档中的局部冲突，并在 Phase 0 先形成正式骨架：

- `contracts/config.user.schema.json` 中 `server.host` 默认值采用 `127.0.0.1`。
  - 原因：3.9.2 明确规定默认只监听本机。
  - 3.10.1.2 中的 `0.0.0.0` 视为参考示例冲突，后续需同步修正文档。
- OneBot 连接地址正式键名采用 `onebot.ws_url`。
  - 原因：3.10.1.2 给出了完整参考结构。
  - 3.1.1 中 `endpoint` 叙述视为历史命名，后续需同步修正文档。
- `global.json` 必须有具体 SDK 版本，因此 Phase 0 锁定为 `10.0.103`。
- `server/go.mod` 由于仓库当前没有 Git remote，Phase 0 先使用临时本地 module path：`rayleabot/server`。

## contracts/ 作为正式来源

以下边界的最终裁决不在 Markdown，而在 `contracts/`：

- 插件 manifest：`contracts/plugin-info.schema.json`
- 插件 JSONL 协议：`contracts/plugin-protocol.schema.json`
- HTTP API：`contracts/web-api.openapi.yaml`
- WebSocket：`contracts/websocket-events.yaml`
- 用户配置：`contracts/config.user.schema.json`
- 错误码：`contracts/error-codes.yaml`
- 发行元数据：`contracts/release-manifest.schema.json`

## Phase 0 非业务 TODO

以下 TODO 不属于业务实现，但必须显式保留：

- `TODO(repo.identity)`：仓库配置正式 remote 后，把 `server/go.mod` 的临时 module path 替换为正式模块路径。
- `TODO(deps.chromium.version)`：在 `.deps/manifest.json` 中补齐受控 Chromium 的正式版本。
- `TODO(deps.source_and_sha256)`：在 `.deps/manifest.json` 中补齐 Chromium、Python、Node.js 资源来源与 SHA256。
- `TODO(go.sqlite.patch)`：在进入 Server 核心骨架阶段前，正式冻结 `modernc.org/sqlite` 的 module patch。
- `TODO(doc.sync.phase0)`：把规划文档中 `server.host` 与 OneBot 键名的冲突说明同步修正。

规则：

- 上述 TODO 未完成前，不得开始依赖这些值的正式运行时/bootstrap 逻辑。
- 若后续有人尝试在未更新 baseline 与 contracts 的情况下先写功能代码，应视为违反仓库治理规则。
