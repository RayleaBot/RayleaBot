# Bot Core

本页说明 RayleaBot 服务进程中的主控制层职责，以及事件分发、命令解析、聊天权限、调度和后台任务的统一规则。

正式接口、事件名、状态名和错误码以 `contracts/` 为准。

## 当前职责

| 模块 | 作用 |
| --- | --- |
| Bridge | 负责 adapter 事件校验、统一事件转换和桥接层观测 |
| Dispatcher | 负责目标选择、命令定向、fan-out 排队和插件返回动作执行 |
| Command Parser | 负责命令前缀匹配、`command` / `args` 提取与定向投递 |
| Permission System | 负责聊天侧黑名单、权限级别与冷却限流 |
| Plugin Manager | 负责发现、注册、启停与生命周期编排 |
| Runtime Manager | 负责插件握手、保活、重载、崩溃恢复与状态同步 |
| Scheduler | 负责定时触发和一次性任务 |
| Capability Grant Manager | 负责插件能力授权与时效过滤 |
| Config Manager | 负责配置读取、校验、覆盖与热更新入口 |
| Logger | 负责统一结构化日志输出 |
| Render Service | 负责模板渲染、结果缓存与 artifact 管理 |

## 事件分发规则

- 订阅以统一 `event_type` 为中心，当前支持精确匹配和 `*` 全量订阅。
- 多个插件命中同一事件时默认 fan-out 分发，不提供停止传播、优先级抢占或“首个处理者获胜”。
- 同一插件的事件投递按 per-plugin queue 顺序执行；队列满时直接丢弃并进入可观测摘要。
- 目标插件不在 `running` 状态时，待投递事件直接丢弃并进入可观测摘要；平台不为插件补投历史事件。
- Dispatcher 是插件返回动作的唯一执行出口；Bridge 不直接执行出站动作。
- 平台持续维护事件丢弃统计，并通过日志、诊断和管理面暴露最近摘要。

## 命令解析与聊天权限

### 命令解析

- 命令前缀来自 `config/user.yaml` 的 `command.prefixes`。
- 命中前缀后，平台把命令名写入 `payload.command`，把参数数组写入 `payload.args`。
- 插件可在 manifest 的 `commands` 字段中声明主命令、别名、说明、示例和权限级别。
- 命令消息优先定向投递给声明该命令的插件；无声明时仍可按消息订阅继续 fan-out。
- `raylea:*` 保留给官方内置插件；第三方插件不得占用。
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

- Scheduler 使用服务主时区；未显式配置时默认跟随宿主机本地时区。
- 周期性任务在服务离线期间不补跑，一次性任务错过窗口后按 `missed` 或 `expired` 处理。
- 同一插件同一 `task_id` 的调度注册按更新处理，不生成重复任务。
- 插件被禁用、卸载或进入 `dead_letter` 后，关联调度任务会暂停或移除。

### 后台任务模型

- 插件安装、重载、备份、恢复、迁移、运行环境准备和渲染预览统一进入后台任务模型。
- 统一任务字段包括 `task_id`、`task_type`、`status`、`progress`、`summary`、`started_at`、`finished_at`、`result` 和 `error`。
- Web UI、CLI、日志和管理 WebSocket 复用同一套任务状态，不再为不同长操作发明独立状态模型。
- 可取消与不可取消的长操作边界由任务类型决定；不可逆阶段不接受假性的“取消中”状态。

## 启动与 Ready 语义

- 启动顺序固定为配置加载、迁移检查、运行时与渲染资源检查、初始化判定、插件注册与调度恢复、管理控制面可用、Adapter 建链。
- 配置、迁移、关键资源检查失败时，服务不会进入 `running`。
- 处于 `setup_required` 时，平台不会加载插件、建立 OneBot11 连接或启动调度。
- 本地管理控制面和关键资源正常时，服务即可进入可管理状态；外部协议链路暂时不可用时，可进入 `degraded`，但不会伪装成完全就绪。
- Ready 判断以本地控制面、关键资源和初始化状态为主，不用 `degraded` 掩盖本地启动失败。

## 相关文档

- [Event Model](./event-model.md)
- [State Model](./state-model.md)
- [Platform Runtime](./platform-runtime.md)
