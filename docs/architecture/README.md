# Architecture Docs

本目录说明 RayleaBot 的内部设计、运行链路和跨层边界。

正式字段、状态名、错误码和协议结构以 `contracts/` 为准；本目录负责解释当前实现中的职责分层和运行语义。

## 阅读入口

| 文档 | 主题 |
| --- | --- |
| [Event Model](./event-model.md) | OneBot11 事件归一化、插件协议消息和管理 WebSocket 事件 |
| [State Model](./state-model.md) | 插件运行时、任务、授权时效和连接状态 |
| [Bot Core](./bot-core.md) | 事件分发、命令解析、聊天权限、调度和后台任务 |
| [Render Service](./render-service.md) | 模板渲染、队列、artifact 与资源边界 |
| [Platform Runtime](./platform-runtime.md) | 配置、存储、日志、恢复、Launcher 控制面和兼容策略 |

## 当前主链路

```plain
OneBot11 WS
  -> adapter
  -> bridge
  -> dispatcher
  -> plugin runtime
  -> local action RPC
  -> outbound / storage / render / scheduler
  -> adapter
  -> OneBot11 WS
```

1. Adapter 接收 OneBot11 事件并转换为统一事件模型。
2. Bot Core 把事件送入 EventBus，并补齐昵称、角色、目标等上下文字段。
3. Command Parser 检查命令前缀，命中后写入 `payload.command` 与 `payload.args`。
4. Permission System 执行聊天侧黑名单、权限级别和冷却限流检查。
5. Plugin Manager 按启用状态和订阅关系选择目标插件。
6. Runtime Manager 把事件投递给插件子进程，并接收结果、错误和本地 action。
7. 需要图片输出时，Bot Core 调用 Render Service 生成或复用缓存 artifact。
8. Bot Core 把插件动作投影到 Adapter、存储、调度或其他平台能力。
9. Web API 暴露状态、任务、配置、日志和诊断；Launcher 在服务可用后复用这些正式入口。

## 跨层边界

- adapter 只负责平台协议适配，不直接写业务状态。
- runtime manager 只通过正式 local action surface 访问平台能力。
- Web 管理面消费正式 HTTP / WebSocket，不靠日志猜状态。
- Launcher 复用服务端管理入口，不维护独立状态模型。
