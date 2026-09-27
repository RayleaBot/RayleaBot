# 插件后台事件实施计划

更新日期：2026-09-27。状态：协议、宿主、Web 配置页与 Go SDK 已实施（第 2～4 节及第 6 节的宿主与 SDK 验收）；插件一次性迁移（第 5 节）与真实账号验收待进行。

## 1. 目标

插件的宿主动作只能挂在未收尾的事件上，事件默认 60 秒超时，同一插件在同一会话的事件按 FIFO 串行。扫码登录等待确认、逐个读取几十个角色面板、抽卡链接翻页这类分钟级流程，因此只能切成不超过 60 秒的片段，再借每分钟的计划任务续跑。这带来了分钟级延迟、续跑状态机、租期与超时恢复、为后台触发签发的账号委托、旧任务清理，以及“触发不带任务 ID 导致定时功能空跑”一类缺陷。

本计划给宿主增加通用的后台事件能力：插件可以先以一个结果结束事件的投递，释放会话队列与并发槽，同时保留一个绑定原事件来源的动作上下文，在有限期限内继续调用宿主。长流程由此写成顺序代码，计划任务只用于真正的周期工作。同时补齐三处相关缺陷：事件期限对插件可见、计划触发携带任务 ID、计划任务的耗时计入排队时间。

参照：Yunzai 与 AstrBot 的插件与框架同进程，处理函数不设时限并可随时回复；本项目插件是独立进程，按事件授权动作，后台事件在保持授权边界的前提下提供等价能力，思路接近 Discord 交互的“先确认、后续消息凭令牌在限时内发送”。

## 2. 协议与契约

### 2.1 `event.detach`

新增本地动作 `event.detach`，由当前事件的 handler 调用：

```json
{ "result": { }, "propagation": "stop" }
```

- `result` 可选，形状与该事件类型终态 `result` 的 `data` 相同（`management.action` 为返回给管理页的结果对象），省略时等同空结果。`propagation` 可选，只用于消息事件，与成功终态帧上的 `propagation` 同义；它与 `result` 并列，不放进 `result`，使管理动作的结果对象保持原样。宿主立即以它们完成该事件的投递：消息事件按传播决定继续分层，管理动作把 `result` 返回给调用方，调度触发记为已投递。非消息事件携带 `propagation` 返回 `platform.invalid_request`。
- 可转入后台的事件类型：`message.private`、`message.group`、`scheduler.trigger`、`management.action`。其他事件（含 `plugin.request`、`webhook.received`、生命周期与控制事件）调用时返回 `platform.invalid_request`。
- 每个事件最多转入一次；重复调用返回 `platform.invalid_request`。
- 动作结果为 `{ "deadline_at_ms": <int> }`，即后台期限：转入时刻加 `runtime.plugin_detached_event_timeout_seconds`。
- 每个插件进程同时处于后台的事件最多 `runtime.max_detached_events_per_plugin` 个，超出返回可重试的 `platform.rate_limited`，事件仍按普通事件继续。

转入后台后：

- 该事件不再占用会话 FIFO 与插件并发槽；后续消息正常投递给插件。
- 事件保留原 `request_id` 与来源（bot、actor、target、调度任务 ID），插件继续以它为 `parent_request_id` 调用宿主动作；`plugin.call` 的 `origin` 仍是原事件来源。
- 禁止 `session.wait`（返回 `platform.invalid_request`），多轮对话只由在前台完成的消息事件登记。
- 插件以终态 `result` 或 `error` 结束后台事件；终态只用于结束与统计，不再参与传播或返回管理页。后台事件不接受“终态动作”（以发送消息兼作终态的写法）或携带 `propagation` 的终态，收到时按协议违规处理；SDK 负责把回复写成普通动作加终态。
- 到期未结束时宿主按 `plugin.event_timeout` 结束事件、拒绝其后续动作，并取消其未完成的 `plugin.call`；插件停止、重载时后台事件按 `plugin.event_canceled` 结束。宿主不重放、不续期。

### 2.2 事件期限

所有 `event` 帧新增 `deadline_at_ms`（Unix 毫秒）：该事件在宿主处的处理期限，即投递时刻加 `runtime.plugin_event_timeout_seconds`；`plugin.request` 沿用服务请求中已有的期限。转入后台后以 `event.detach` 返回的期限为准。

### 2.3 调度触发携带任务 ID

`scheduler.trigger` 事件的 `payload` 新增必填 `task_id`（`scheduler.create` 时的任务 ID），与已有的 `payload`、`action` 并列。插件按它分派触发；收到无法识别的任务 ID 时应删除该任务。

### 2.4 配置

`runtime` 新增：

| 键 | 默认 | 范围 | 说明 |
| --- | --- | --- | --- |
| `plugin_detached_event_timeout_seconds` | 900 | 60–3600 | 后台事件期限 |
| `max_detached_events_per_plugin` | 8 | 1–64 | 每个插件进程同时处于后台的事件上限 |

均可热更新；已转入后台的事件保留转入时的期限。

## 3. 宿主实现

1. 运行时事件会话增加“后台”状态：`event.detach` 校验事件类型与上限后，以给定结果完成投递并返回，会话改用不随投递上下文取消的独立上下文和后台期限计时，继续接受动作；终态、超时、进程停止时结束并释放。
2. 调度分派与分层传播使用转入时的结果；调度统计在后台事件真正结束时记录结果与耗时。
3. 事件帧写入 `deadline_at_ms`；调度触发的 `PayloadFields` 已含 `task_id`（3dad58c7），投影到插件事件的 `payload.task_id`。
4. 调度统计的耗时从触发时刻计算，包含排队时间。
5. 日志：后台事件转入、结束、超时各记一条（插件、事件类型、来源、期限、耗时），超时为 WARN；不记录正文。
6. 契约、fixtures、协议生成物（`scripts/generate-plugin-wire.py`）、运行时 schema 镜像、配置 schema 与默认配置、协议与配置文档、0.7.0 候选说明同步更新。本计划期间新增的宿主能力都算入核心 0.7.0，协议仍为 v4，不设版本门槛。

## 4. Go SDK

- `EventContext.Detach(ctx, result any) (time.Time, error)`：发送 `event.detach`，返回后台期限；消息事件用 `DetachWithPropagation(ctx, result, propagation)` 同时决定传播。
- handler 的 context 在事件期限到达时取消；转入后台后改按后台期限。`EventContext.Deadline()` 返回当前期限。
- 转入后台后，`SendText`、`Send`、`Reply` 以普通消息动作发送，再以终态 `result` 结束；`Result`、`Fail` 照常作为终态。
- `Event.TaskID()` 读取调度触发的任务 ID。

## 5. 插件一次性迁移

四个插件在宿主与 SDK 合入后一次改完，不保留兼容路径：

- **长流程改为后台事件的顺序代码**：绝区零更新面板逐个读取角色、三款游戏的抽卡链接翻页与抽卡记录更新、原神充值记录读取、下载全部资源、账号插件的聊天扫码登录（发码、30 秒撤回、每 5 秒查询至二维码过期、结果图）。管理页发起的同类长任务使用 `management.action` 转入后台。
- **删除为续跑存在的代码**：共用的聊天任务续跑实现、租期与超时恢复、40/55 秒预算、每分钟的续跑与扫码通知任务、为续跑签发的账号委托（如 `zzz.character`）、在 payload 中写入 `task_id` 的写法、对“payload 缺 `task_id` 的旧任务”的重建逻辑，以及对应测试与契约段落。
- **调度触发**：按 `payload.task_id` 分派；无法识别的任务在触发时删除自身。周期任务（提醒、签到、推送、月报收集、社区任务等）继续使用计划任务与按需的账号委托。
- 宿主数据库中已有的续跑类任务由各插件的“无法识别即删除”在首次触发时清除，无需单独迁移脚本；插件数据目录中续跑留下的状态文件不再读取。

## 6. 验收

- 宿主：后台事件的转入、动作、终态、超时、停止与重载，事件类型与次数限制，上限拒绝，传播与管理结果，`session.wait` 拒绝，调度统计与排队计时，`deadline_at_ms` 与 `payload.task_id` 的投影；契约门禁、生成物校验与 server 全量测试。
- SDK：`Detach`、期限 context、后台回复写法、`TaskID`。
- 插件：每个长流程的协议级测试（经真实 SDK 运行时与宿主帧形状），一次事件内完成全程并回复；各仓库 `gofmt`、`go vet`、`go test`、`deadcode` 通过。
- 真实账号验收：绝区零更新面板、扫码登录、抽卡链接在开发实例上各走一遍。
