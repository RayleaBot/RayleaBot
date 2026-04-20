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
OneBot11 transport
  -> adapter
  -> event ingress
  -> bridge
  -> dispatcher
  -> plugin runtime
  -> local action service
  -> outbound / storage / render / scheduler
  -> adapter
  -> OneBot11 transport
```

1. Adapter 负责 OneBot11 transport、协议帧解析和统一事件归一化。
2. Event Ingress 负责命令提取、聊天权限、冷却回复、reply target 记录和 adapter ready 协调。
3. Bridge 校验统一事件结构，补齐桥接层观测字段，并把事件交给 Dispatcher。
4. Dispatcher 只选择处于 `running` 的插件 runtime，按命令声明或事件订阅关系 fan-out 排队。
5. Runtime Manager 把事件投递给插件子进程，并接收结果、错误和本地 action。
6. Local Action Service 提供配置、存储、调度、渲染、Webhook 暴露和 OneBot 动作访问。
7. Dispatcher 执行插件返回的出站动作；Render Service 负责图片 artifact 生成与复用。
8. Protocol Service、Plugin Webhook Service、System Service 与 HTTP / WebSocket handlers 暴露管理入口和诊断视图。

## 当前组装边界

| 组件 | 职责 |
| --- | --- |
| App | 负责组装、运行、关闭和统一 `http.Handler` 输出 |
| Event Ingress Service | 负责 adapter 事件入口、命令提取、聊天权限和 ready 协调 |
| Bridge | 负责统一事件校验与桥接层观测 |
| Dispatcher | 负责目标选择、fan-out 排队和插件返回动作执行 |
| Plugin Lifecycle Controller | 负责插件启停、重载、崩溃恢复、Dispatcher 注册和调度触发 |
| Runtime Registry / Manager | 负责单插件进程握手、保活、停止和崩溃状态 |
| Local Action Service | 负责插件本地动作执行和平台能力访问 |
| Protocol Service | 负责协议快照、reverse-ws / webhook 协议入口和协议事件推送 |
| Plugin Webhook Service | 负责插件 webhook 注册、鉴权、按需拉起和定向投递 |
| System Service | 负责恢复摘要、运行环境诊断、备份和系统状态 |
| Capability Grant Manager | 负责插件能力授权与时效过滤 |
| Governance Service | 负责黑白名单、白名单状态、命令策略读取面与聊天侧权限裁决 |
| HTTP / WebSocket Handlers | 负责按领域暴露管理 API 与实时通道 |

## 跨层边界

- adapter 只负责平台协议适配，不直接写业务状态。
- event ingress 负责进入 bridge 前的命令与聊天权限处理。
- runtime manager 只通过正式 local action surface 访问平台能力。
- protocol、plugin webhook、system 和 local action 各自持有窄依赖，不经由 `*App` 互相穿透。
- Web 管理面消费正式 HTTP / WebSocket，不靠日志猜状态。
- Launcher 复用服务端管理入口，不维护独立状态模型。
