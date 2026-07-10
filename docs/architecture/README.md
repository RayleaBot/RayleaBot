# Architecture Docs

本目录说明 RayleaBot 的组件 owner、信任边界、状态源和运行链路。字段、状态、错误码和协议结构以 `contracts/` 为准。

## 阅读入口

| 文档 | 主题 |
| --- | --- |
| [Platform Architecture](./platform-architecture.md) | 组件 owner、信任边界、状态源、部署边界和代码地图 |
| [Message Flow](./message-flow.md) | OneBot11 入站、插件分发、local action、出站、调度与 webhook |
| [Event Pipeline](./event-pipeline.md) | adapter、ingress、policy、bridge、dispatch、runtime 与 outbound |
| [Event Model](./event-model.md) | OneBot11 事件、插件协议消息和管理 WebSocket 事件 |
| [State Model](./state-model.md) | 插件 runtime、任务和连接状态 |
| [Server Lifecycle](./server-lifecycle.md) | 启动、运行、关闭和依赖组装 |
| [Management API](./management-api.md) | 管理 API、鉴权、错误和 contract 同步 |
| [Plugin Runtime](./plugin-runtime.md) | 插件进程、事件投递和 local action 边界 |
| [Bot Core](./bot-core.md) | 命令、治理、调度和后台任务 |
| [Render Service](./render-service.md) | 模板、Chromium、artifact 与资源摘要 |
| [Platform Runtime](./platform-runtime.md) | 配置、存储、日志、恢复和 Launcher 控制 |
| [Storage Migrations](./storage-migrations.md) | SQLite schema 与 migration 不变量 |
| [Technology Decisions](./technology-decisions.md) | 正式技术栈和依赖替换准则 |

## 架构不变量

- `contracts/` 裁决所有对外边界。
- Server 是在线状态源；Web 与 Launcher 只持有临时投影。
- Adapter、Event Ingress、Bridge、Dispatcher、Runtime Manager 和 Local Action Service 各有单一职责。
- 插件代码信任、浏览器会话、Launcher control 和发布更新是独立信任边界。
- Tasks、Scheduler 和 updater 各自使用单一 mutation path 与持久化终态。
- 客户端、插件和文档不能建立与 server 或 contracts 竞争的语义来源。

从 [Platform Architecture](./platform-architecture.md) 进入组件与信任总览，从 [Message Flow](./message-flow.md) 进入事件链路。
