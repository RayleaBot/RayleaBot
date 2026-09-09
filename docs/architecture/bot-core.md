# Bot Core

本页说明 RayleaBot 服务进程中的主控制层职责，以及事件分发、命令解析、聊天权限、调度和后台任务的统一规则。

正式接口、事件名、状态名和错误码以 `contracts/` 为准。

## 当前职责

| 模块 | 作用 |
| --- | --- |
| App | 负责服务组装、运行控制、关闭和统一路由输出 |
| Chat Policy Ingress | 位于 `eventpipeline/chatpolicy`，负责 adapter 事件入口、命令提取、聊天权限、cooldown reply 和 adapter ready 协调 |
| Bridge | 负责 adapter 事件校验、统一事件转换和桥接层观测 |
| Dispatcher | 负责目标选择、命令定向、fan-out 排队和插件返回动作执行 |
| Plugin Lifecycle Controller | 负责发现、注册、启停、重载、崩溃恢复和生命周期编排 |
| Runtime Manager | 负责插件握手、保活、重载、崩溃恢复与状态同步 |
| Local Action Service | 负责消息、配置、secret、存储、插件目录、三方账号、治理、渲染、调度、Webhook、OneBot 与 provider 动作；完整清单见[插件协议](../plugin/protocol.md#action-rpc) |
| Protocol Service | 负责协议快照、OneBot 回连入口和 Webhook 协议入口 |
| Plugin Webhook Service | 负责插件 webhook 注册、鉴权、按需拉起和事件投递 |
| Scheduler | 负责 cron 周期任务的注册与定时触发 |
| Permission View | 提供插件权限与参数查询 |
| Config Manager | 负责配置读取、校验、覆盖与热更新入口 |
| Logger | 负责统一结构化日志输出 |
| Render Service | 负责模板渲染、结果缓存与 artifact 管理 |

## 事件分发规则

- `eventpipeline/chatpolicy` Ingress 在进入 Bridge 前完成命令提取、黑名单、权限级别和冷却限流检查。
- 订阅以统一 `event_type` 为中心，当前支持精确匹配和 `*` 全量订阅。
- 多个插件命中同一事件时默认 fan-out 分发，不提供停止传播、优先级抢占或“首个处理者获胜”。
- 同一插件的事件先进入 per-plugin queue，再按 `event.target` 切分 lane。
- 同一 lane 保持 FIFO，不同 lane 在插件有效并发度内并发执行。
- 队列满时直接丢弃并进入可观测摘要。
- 目标插件不在 `running` 状态时，待投递事件直接丢弃并进入可观测摘要；平台不为插件补投历史事件。
- Dispatcher 是插件返回动作的唯一执行出口；Bridge 不直接执行出站动作。
- 平台持续维护事件丢弃统计，并通过日志、诊断和管理面暴露最近摘要。

## 命令解析与聊天权限

### 命令解析

- 命令前缀来自 `config/user.yaml` 的 `command.prefixes`。
- `eventpipeline/chatpolicy` Ingress 命中前缀后，把命令名写入 `payload.command`，把参数数组写入 `payload.args`。
- 插件可在 manifest 的 `commands` 字段中声明主命令、别名、说明、示例和权限级别。
- 命令消息优先定向投递给声明该命令的插件；无声明时仍可按消息订阅继续 fan-out。
- 默认官方插件当前使用 `raylea.` ID 前缀；插件身份仍以来源记录和 `info.json.id` 的一致性校验为准。
- 非命令消息不受命令路由影响，继续按消息事件分发。

### 聊天权限

| 级别 | 含义 |
| --- | --- |
| `super_admin` | 仅超级管理员可用 |
| `group_admin` | 群主、群管理员和超级管理员可用 |
| `everyone` | 所有用户可用 |

- 超级管理员列表来自 `admin.super_admins`。
- 群管理员角色由事件归一化阶段补齐到 `actor.role`。
- 用户黑名单和群黑名单会在事件分发前生效。
- 超级管理员保留最终人工干预通道，不受聊天侧黑名单拦截。
- 平台内建用户侧冷却限流；权限不通过时可返回受控短提示。

## 调度与后台任务

### Scheduler

- Scheduler 使用 `scheduler.timezone` 的 IANA 时区，默认 `Asia/Shanghai`（UTC+08:00），旧配置留空时采用同一默认值。管理面展示与日志筛选使用配置接口返回的 `effective_timezone`；保存新时区后，服务重启前仍使用当前生效值。重启时按新时区重算未来任务，积压任务保留恢复行为。
- Scheduler 只接受五段 cron 周期表达式，不负责一次性长操作。
- 周期性任务在服务离线期间不补跑，恢复后按下一个匹配时间点触发。
- 同一插件同一 `task_id` 的调度注册按更新处理，不生成重复任务。
- 插件被禁用、卸载或进入需要人工恢复的失败状态后，关联调度任务会暂停或移除。

### 后台任务模型

- Task Registry 是有限长异步操作的职责方，负责 admission、执行状态、持久化和关闭 drain。
- 当前后台任务固定为 `plugin.install`、`plugin.uninstall`、`plugin.reload`、`backup.create`、`restore.apply`、`recovery.recheck`、`recovery.confirm`、`runtime.bootstrap`。
- 数据库 schema 迁移在存储启动时同步执行；模板 HTML 预览是同步接口，二者都不创建后台任务。
- 统一任务字段包括 `task_id`、`task_type`、`status`、`progress`、`summary`、`started_at`、`finished_at`、`result` 和 `error`。
- Web UI、CLI、日志和管理 WebSocket 复用同一套任务状态，不为不同长操作发明独立状态模型。
- 可取消与不可取消的长操作边界由任务类型决定；不可逆阶段不接受假性的“取消中”状态。

## 启动与 Ready 语义

- App 负责配置加载、平台服务组装、插件服务组装、HTTP 路由注册和关闭协调。
- 启动检查覆盖迁移、运行时资源、渲染资源、初始化判定、插件注册与调度恢复，以及 Adapter 建链。
- 配置、迁移、关键资源检查失败时，服务不会进入 `running`。
- 处于 `setup_required` 时，平台不会加载插件、建立 OneBot11 连接或启动调度。
- 本地管理控制面和关键资源正常时，服务即可进入可管理状态；外部协议链路暂时不可用时，可进入 `degraded`。
- Ready 判断以本地控制面、关键资源和初始化状态为主，不用 `degraded` 掩盖本地启动失败。

## 相关文档

- [Event Model](./event-model.md)
- [State Model](./state-model.md)
- [Platform Runtime](./platform-runtime.md)
