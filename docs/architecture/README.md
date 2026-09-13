# Architecture Docs

本目录说明 RayleaBot 各组件的职责归属、信任边界、状态来源和运行链路。字段、状态、错误码和协议结构以 `contracts/` 为准。

## 阅读入口

| 文档 | 主题 |
| --- | --- |
| [Platform Architecture](./platform-architecture.md) | 组件职责归属、信任边界、状态来源、部署边界和代码地图 |
| [Message Flow](./message-flow.md) | 聊天适配器入站、插件分发、local action、出站、调度与 webhook |
| [Event Model](./event-model.md) | OneBot11、QQ 官方事件、插件协议消息和管理 WebSocket 事件 |
| [State Model](./state-model.md) | 插件 runtime、任务和连接状态 |
| [Server Lifecycle](./server-lifecycle.md) | 启动、运行、关闭和依赖组装 |
| [Plugin Runtime](./plugin-runtime.md) | 插件进程、事件投递和 local action 边界 |
| [Bot Core](./bot-core.md) | 命令、治理、调度和后台任务 |
| [Render Service](./render-service.md) | 模板、Chromium、artifact 与资源摘要 |
| [Platform Runtime](./platform-runtime.md) | 配置、存储、日志、恢复和 Launcher 控制 |

## 架构不变量

- Server 是在线的状态来源；Web 与 Launcher 只保存临时视图。
- Adapter、`bot/pipeline/chatpolicy` Ingress、Bridge、Dispatcher、Runtime Manager 和 Local Action Service 各有单一职责。
- 插件代码信任、浏览器会话、Launcher control 和发布更新是独立信任边界。
- Tasks 和 Scheduler 各自只有唯一的写入路径，并把最终状态持久化。

从 [Platform Architecture](./platform-architecture.md) 进入组件与信任总览，从 [Message Flow](./message-flow.md) 进入事件链路。
