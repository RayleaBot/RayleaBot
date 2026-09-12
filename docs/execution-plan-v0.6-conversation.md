# RayleaBot v0.6 多轮会话、优先级阻断与 KV TTL 执行计划

## 文档状态

- 目标版本：v0.6
- 执行范围：`contracts/`、`server/`、`sdk/go`、`web/` 插件详情、`fixtures/`、`examples/plugins/` 与相关文档
- 升级方式：协议仍为 v3、manifest 仍为 v3，全部为可选字段与新增动作；SQLite 结构升到 `000002` 并首次引入前向迁移
- 当前状态：方案已定，待维护者确认默认值后开始实施
- 验收方式：本文档保留，逐项验收后整理为 `docs/CHANGELOGS/v0.6.md`

状态说明：

- `⬜ 待处理`：尚未开始。
- `🟡 进行中`：已经开始，但尚未满足完成条件。
- `☑️ 已完成`：实现、配套更新和本项验证均已完成。
- `❌ 阻塞`：当前范围内无法完成，已记录原因。

## 一、现状与问题

以下事实来自当前 `main`，是后文决策的依据。

| 主题 | 当前实现 | 直接后果 |
| --- | --- | --- |
| 事件派发 | `bot/pipeline/dispatch/delivery.go` 的 `selectTargets`：命令消息定向给所有声明该命令的插件，否则按 `events` 订阅全量 fan-out；`enqueueTargets` 只入队不等待终态，终态由 `worker.go` 的 `deliverLaneItem` 消费后不再回馈派发决策 | 没有优先级，也没有"高优先级插件消费后阻止后续插件"的信息通道 |
| 终态帧 | `result` / `action` / `error` 三种终态；`plugin.not_handled` 错误码已登记（`contracts/error-codes.yaml`）但宿主不据此做任何路由决定；任何 `error` 终态都进入失败计数并打 Warn | 插件无法表达"我不处理，请继续"或"我已处理，请停止" |
| 命令与治理 | `chatpolicy/policy_service.go` 的 `Apply` 在同一步完成命令解析、黑白名单、命令权限与冷却；`permission.Checker.Check` 在 `cmd == nil` 时只做超管旁路和黑名单，不做冷却 | 用户回复的"1"不带前缀，不会被识别为命令，也无法定向到发问插件；但非命令消息天然不受冷却影响，这正是会话回复需要的路径 |
| 事件生命周期 | 每个事件是一次请求-响应，`runtime.plugin_event_timeout_seconds` 默认 60 秒；同一 `event.target` 的事件在同一 lane 内 FIFO | 插件不能在事件内阻塞等待下一条消息：后续消息会排在等待中的事件之后，形成死锁 |
| Local action 上下文 | `plugins/actions/dispatch.go` 的 `ActionRequest` 携带 `ParentEvent` | 宿主可以从父事件推导会话键，插件登记等待时不必重复传会话身份 |
| KV | `plugin_kv(plugin_id, key, value_json, size_bytes, updated_at)`；`set` 只有 `key` 与 `value`；配额按 `SUM(size_bytes)` 统计 | 验证码、会话锁等临时数据必须由插件自行清理 |
| 数据库结构版本 | `storage/store_schema.go` 只接受 `000001`，无迁移机制；`recovery.go` 与 `backup-manifest.schema.json` 只接受当前版本 | 任何结构变更都需要先建立前向迁移，否则升级后现有安装无法启动、旧备份无法恢复 |
| 管理面 | 同名命令被多个插件声明时标记 `command_conflicts`，运行时仍全量投递 | 冲突提示只是信息，没有可预测的先后顺序 |

## 二、同类项目对照

| 能力 | NoneBot2 | Koishi | AstrBot | Yunzai (Miao-Yunzai) | 本计划 |
| --- | --- | --- | --- | --- | --- |
| 多轮等待 | `got` / `receive` / `reject` / `pause`：暂停时复制一个临时 matcher（`temp=True`、`priority=0`、`block=True`，到期时间为 `SESSION_EXPIRE_TIMEOUT`，默认 2 分钟），会话键为群 + 用户，`permission_updater` 可扩展为多用户 | `session.prompt(timeout)` 基于临时中间件，只对同用户同频道生效，默认超时 `delay.prompt` 为 60 秒，超时返回 `null` | `@session_waiter(timeout, record_history_chains)`，会话键默认 `sender_id`，`SessionFilter` 可改为群；`controller.keep / stop`；超时抛 `TimeoutError`；等待期间消息先经 waiter，不经命令解析 | `setContext(type, isGroup, time=120, timeout 文案)`，键为插件名 + 群号或用户号；上下文在规则匹配前处理；`finish` 结束；超时自动回复文案 | 宿主登记式：`session.wait` 登记后正常结束当前事件，匹配的后续消息作为新事件定向投递并附 `payload.session`；`scope` 为 `user`（默认）或 `conversation`；默认 60 秒、上限可配；`max_turns`、`state` 回显、`session.expired` 通知；SDK 另提供阻塞式 `Prompt` 糖 |
| 优先级 | `priority` 数字小者先，默认 1；同级按注册顺序并发 | 中间件按注册顺序，`prepend` 前置 | `priority` 数值大者先，默认 0 | `priority` 数字小者先，默认 5000 | manifest `priority` 整数，数值大者先，默认 0，范围 -1000..1000；同级并发 |
| 阻断 | 静态 `block`（非命令 message matcher 默认 `True`）+ 动态 `stop_propagation()` | 不调用 `next()` 即阻断 | 动态 `event.stop_event()` | 处理函数返回非 `false` 即阻断（默认阻断） | 静态 manifest `block`（默认 `false`）+ 终态帧 `propagation` 动态覆盖；`error` 终态、超时、丢弃一律继续 |
| KV TTL | 无内置 KV | `ctx.cache` 的 `maxAge` | 无 TTL | Redis `EX` | `storage.kv set` 新增 `ttl_seconds` 与 `if_not_exists`；读时过滤、后台清扫；配额排除过期行 |

参考来源：[NoneBot2 事件响应器进阶](https://nonebot.dev/docs/advanced/matcher)、[NoneBot2 会话控制](https://nonebot.dev/docs/appendices/session-control)、[NoneBot2 会话更新](https://nonebot.dev/docs/advanced/session-updating)、[Koishi 中间件](https://koishi.chat/zh-CN/guide/basic/middleware.html)、[Koishi Session API](https://koishi.chat/zh-CN/api/core/session.html)、[AstrBot 会话控制](https://docs.astrbot.app/dev/star/guides/session-control.html)、[AstrBot 处理消息事件](https://docs.astrbot.app/dev/star/guides/listen-message-event)、[Miao-Yunzai plugin.js](https://github.com/yoimiya-kokomi/Miao-Yunzai/blob/master/lib/plugins/plugin.js)、[Miao-Yunzai loader.js](https://github.com/yoimiya-kokomi/Miao-Yunzai/blob/master/lib/plugins/loader.js)。

四个项目的等待机制都只保存在内存中，宿主重启即失效；本计划同样不持久化等待项。

## 三、固定决策

1. 会话等待采用宿主登记模型，不延长事件会话。插件在当前事件内通过 local action 登记，然后照常发送终态；命中的后续消息作为新事件定向投递给登记插件，`event_type` 不变，附 `payload.session`。协议层没有阻塞式等待，SDK 的阻塞式 `Prompt` 只是进程内续体。
2. 会话键为 `plugin_id + source_protocol + source_adapter + target.type + target.id`，`scope=user` 时再加 `actor.id`。默认 `user`；`conversation` 允许同会话任何人回复。私聊时两种 scope 等价。
3. 匹配发生在 Ingress 命令解析之前，只作用于 `message.private` / `message.group`。命中后只保留超管旁路与黑名单（即 `Checker.Check(cmd=nil)`），跳过白名单、命令权限、冷却与内置菜单。事件仍填充 `payload.command` / `payload.args`（若形如命令），便于插件识别 `/cancel` 之类输入，但不再定向给其他命令声明者。
4. 会话回复对登记插件独占：不进入优先级分层，也不 fan-out。多个等待项同时命中时，`user` 作用域优先于 `conversation`，同作用域取最近登记者，其余等待项保留到各自过期。
5. `max_turns` 默认 1（一次性，NoneBot / Koishi 语义），最大 100；插件处理回复时可用同一 `session_id` 再次 `session.wait`（AstrBot `keep` 语义：重置超时、累加 `turn`）；`session.finish` 提前结束。一次性等待项在被消费到重新登记之间到达的消息走普通链路，文档写明。
6. `timeout_seconds` 默认 `runtime.session_wait_default_seconds`（60），上限 `runtime.session_wait_max_seconds`（600）。过期时仅当登记了 `notify_on_expire` 才投递 `session.expired` 事件；宿主不代发超时文案，出站仍由插件负责。
7. 每插件活跃等待项上限 `runtime.session_wait_max_active_per_plugin`（256），超限返回 `platform.rate_limited`；同键重复登记替换旧项（不通知）。插件停止、重载、禁用或进入失败状态时清空其等待项；宿主不持久化等待项。
8. `state` 为 JSON object，序列化后不超过 4096 字节，超限返回 `platform.value_too_large`；宿主原样回显在每个后续事件的 `payload.session.state`。更大的状态用 KV。
9. manifest 新增 `priority`（整数，-1000..1000，默认 0）与 `block`（布尔，默认 `false`）。数值大者先；同级并发，保持现有 fan-out 行为；分层链式投递，上一层全部终态后再投下一层，任一插件要求停止则终止。终态 `result` / `action` 帧可选 `propagation: "stop" | "continue"` 覆盖 manifest 默认；`error` 终态（含 `plugin.not_handled`）、事件超时、队列满、运行时不可投递均视为继续。
10. 命令定向集合与订阅 fan-out 集合各自排序，命令定向仍优先且独占，与现状一致。分层不改变 lane FIFO；下一层的入队在上一层终态回调中执行；上一层某插件耗尽 `plugin_event_timeout_seconds` 会等量延迟下一层。
11. `plugin.not_handled` 不再进入失败计数与 Warn 日志，作为正常终态记录到 Debug。
12. `storage.kv set` 新增 `ttl_seconds`（≥ 1，≤ 31536000）与 `if_not_exists`；结果返回 `stored` 与可选 `expires_at`（Unix 秒）；`get` 结果新增可选 `expires_at`；`list` 不含过期键。过期判定以宿主时钟在读路径过滤，另有每 60 秒、每批最多 1000 行的后台清扫；配额统计排除过期行。不新增 `incr` 等原子计数动作。
13. `plugin_kv` 新增 `expires_at_ms INTEGER NULL` 与部分索引，结构版本升为 `000002`，并建立首个前向迁移：启动时按 `schema_metadata.version` 顺序执行迁移并在事务内更新版本；恢复接受可前向迁移的旧结构，由启动时迁移完成升级。这是本计划唯一的横切改动，先于 KV TTL 落地。
14. 契约兼容：协议版本保持 `3`。旧 SDK 构建的插件不会登记等待项，因而不会收到 `payload.session` 或 `session.expired`；使用新能力的插件应声明 `min_core_version >= 0.6.0`。Web API `info.version` 递增 patch。
15. 不做：跨插件修改事件或中间件链、会话持久化、管理面手动终止会话、全局取消关键词、原子计数动作、按事件类型等待 notice 事件、管理员在管理面覆盖插件优先级。这些在验收后按需要单独立项。

命名约定：协议中既有的"进程会话"（init 建立）与"事件会话"（一次 `event` 请求）保持原名；本计划新增的概念统一称"对话会话"（conversation session），协议动作与字段使用 `session.*` / `payload.session`，文档首次出现时注明区别。

## 四、契约变更

### `contracts/plugin-info.schema.json`

- 顶层新增 `priority`（`integer`，`minimum: -1000`，`maximum: 1000`，`default: 0`）与 `block`（`boolean`，`default: false`）。
- fixtures：`ok.commands-and-permissions.json` 增加两个字段；新增 `invalid.priority-out-of-range.json`。

### `contracts/plugin-protocol.schema.json`

- `event.event_type` 枚举新增 `session.expired`。
- `event.payload` 新增 `session` 对象（`additionalProperties: false`）：

```json
{
  "session_id": "sess-01J...",
  "scope": "user",
  "turn": 1,
  "state": {"step": "pick_role"}
}
```

- `result` 与 `action` 帧新增可选 `propagation`（`enum: ["stop", "continue"]`）。非终态 action 上出现时忽略，文档写明。
- 新增 `action_session_wait`（`action: "session.wait"`）：

```json
{
  "type": "action",
  "request_id": "act-1",
  "parent_request_id": "evt-1",
  "action": "session.wait",
  "data": {
    "timeout_seconds": 60,
    "scope": "user",
    "max_turns": 1,
    "notify_on_expire": true,
    "state": {"step": "pick_role"},
    "session_id": "sess-01J..."
  }
}
```

  `data` 全部字段可选；`session_id` 仅用于延续既有等待项。结果 `{"session_id": "...", "expires_at": 1757650000, "turn": 0}`。

- 新增 `action_session_finish`（`action: "session.finish"`，`data: {"session_id": "..."}`），结果 `{"finished": true|false}`。
- `action_storage_kv_set_data` 新增 `ttl_seconds`（`integer`，`minimum: 1`，`maximum: 31536000`）与 `if_not_exists`（`boolean`）。
- `session.*` 属于隐式插件私有动作，不进入 `permission_name`。
- fixtures：`ok.session-wait.yaml`（登记、终态、带 `payload.session` 的后续事件、带 `propagation` 的终态）、`ok.session-finish.yaml`、`ok.session-expired.yaml`、`invalid.session-wait-scope.yaml`、`ok.result-propagation.yaml`、`ok.storage-kv-ttl.yaml`、`invalid.storage-kv-ttl-zero.yaml`；`x-fixtures` 同步登记。

### `contracts/error-codes.yaml`

不新增错误码。复用：`platform.invalid_request`（scope、参数非法）、`platform.resource_not_found`（`finish` 未知 `session_id`）、`platform.rate_limited`（等待项超限）、`platform.value_too_large`（`state` 超限）。

### `contracts/config.user.schema.json`

`runtime` 新增三个键，默认值内嵌在 `server/internal/config/default_document.go`，`schema_version` 保持 `4`（实施时以 `default_roundtrip_test.go` 确认新增带默认值的键不需要升版）：

| 键 | 默认 | 说明 |
| --- | --- | --- |
| `session_wait_default_seconds` | 60 | 未指定 `timeout_seconds` 时的等待时长 |
| `session_wait_max_seconds` | 600 | 单次等待上限，超出按上限截断 |
| `session_wait_max_active_per_plugin` | 256 | 每插件活跃等待项上限 |

三个键在登记时读取当前配置，归类为 applied-now。

### `contracts/backup-manifest.schema.json`

`db_schema_version` 枚举新增 `000002`；对应 fixtures 更新。

### `contracts/web-api.openapi.yaml` 与 `contracts/websocket-events.yaml`

- 插件详情与摘要 schema 新增 `priority`、`block`（只读投影）。
- `info.version` 由 `0.3.0` 递增为 `0.3.1`；fixtures 与 examples 同步。

### 生成物

`python scripts/generate-plugin-wire.py`、`node scripts/generate-runtime-schemas.mjs`、`python scripts/generate-error-codes.py`（无变化，仅 `--verify`）、`web` 的 `pnpm run generate:types`、`python scripts/generate-launcher-api.py --verify`（插件字段不在 Launcher 子集内，预期无变化）。

## 五、技术设计

### 5.1 对话会话等待

新包 `server/internal/bot/pipeline/sessionwait`：

```go
type Key struct {
    PluginID, SourceProtocol, SourceAdapter string
    TargetType, TargetID string
    ActorID string // scope=conversation 时为空
}

type Waiter struct {
    ID             string
    Key            Key
    Scope          string
    ExpiresAt      time.Time
    MaxTurns, Turn int
    State          map[string]any
    NotifyOnExpire bool
}

type Registry struct{ /* mu、按 Key 索引、按 PluginID 索引、按 ExpiresAt 的最小堆 */ }

func (r *Registry) Register(ctx context.Context, req RegisterRequest) (Waiter, error)
func (r *Registry) Match(event chatevent.NormalizedEvent) (chatevent.SessionRef, bool)
func (r *Registry) Finish(pluginID, sessionID string) bool
func (r *Registry) CancelPlugin(pluginID string) int
func (r *Registry) Run(ctx context.Context) // 过期清扫，触发 Notifier
```

- `Register` 从 `ActionRequest.ParentEvent` 推导 `Key`；父事件不是消息事件或缺少 target / actor 时返回 `platform.invalid_request`。`session_id` 存在且属于同一插件时延续：重置 `ExpiresAt`，保留 `Turn`，替换 `State`。
- `Match` 原子完成：查找（`user` 优先，再 `conversation`）、`Turn++`、达到 `MaxTurns` 时移除；返回 `chatevent.SessionRef{ID, Scope, Turn, State}`。
- 过期项若 `NotifyOnExpire`，通过注入的 `Notifier` 构造 `event_type=session.expired` 的 `chatevent.Event`（`Target` 与 `Actor` 来自 `Key`，`PayloadFields["session"]` 同结构）并调用 `Dispatcher.DispatchToPlugin`；插件不可投递时按 drop 记录。
- 所有共享状态由单一互斥锁保护；`Match` 在消息热路径上只做一次 map 查找。

`chatevent`：`NormalizedEvent` 与 `Event` 新增 `Session *SessionRef`，`FromAdapter` 透传；`runtime/manager_delivery.go` 的 `buildEventPayload` 输出 `payload.session`。

Ingress（`chatpolicy/ingress.go`）：

```text
enrich metadata → replyTargets.Record
→ if sessions.Match(event) 命中：
     event.Session = ref
     policy.ApplyBaseline(ctx, event)   // 超管旁路 + 黑名单；不解析策略、不冷却
     policy.EnrichCommandEvent(event)   // 仅填充 command/args
     bridge.HandleAdapterEvent(ctx, event)   // 不经内置菜单
→ 否则走现有链路
```

Dispatcher：`Dispatch` 看到 `event.Session != nil` 时目标只有 `Session.PluginID`（由 Ingress 写入 `SessionRef.PluginID`，不进入协议 payload）；插件不可投递时以 `session_plugin_unavailable` 记 drop。

Local action：`plugins/actions/session.go` 注册 `session.wait` 与 `session.finish`，`runtime/actions.go` 解析新帧类型；`Deps` 新增 `Sessions SessionRegistry`。

生命周期：`plugins/lifecycle` 在停止、重载 swap 完成、禁用与进入失败状态时调用 `CancelPlugin`。

日志与观测：`component=sessionwait` 记录登记、命中、延续、结束、过期与清空（含 `plugin_id`、`session_id`、`scope`、`turn`）；`MetricsObserver.IncEventPipelineStage("session", "matched"|"expired")`；`dispatcher_runtime` 的 drop 原因新增 `session_plugin_unavailable`。

平台限制：QQ 官方群消息只有 @ 机器人才会上报，因此群内回复仍需 @；私聊不受影响。文档写明。

### 5.2 优先级与阻断

- `plugins/catalog/manifest.go` 解析 `priority` / `block` 到 `plugins.Snapshot`；`runtime.Spec` 与 `dispatch.SwapPlugin` 增加两个参数，`pluginSlot` 保存。
- `plugins.Delivery` 新增 `Propagation string`；`runtime/manager_delivery.go` 的 `decodeTerminalResult` / `decodeTerminalAction` 读取帧字段；`error` 终态不读取。
- `selectTargets` 返回 `[][]string`：命令定向集合或订阅集合按 `priority` 降序分层，层内按 `plugin_id` 升序；只有一层时行为与现在完全相同。
- `Dispatch` 为多层事件创建 `propagation{remaining int32, stopped atomic.Bool, next func()}`，通过 `dispatchItem.onDone(outcome)` 回调驱动：`deliverLaneItem` 返回后在同一 goroutine 调用 `onDone`，最后一个完成者若未停止则入队下一层。停止判定：`Delivery.Propagation == "stop"`，或 `Propagation == ""` 且插件 `block=true` 且终态为 `result` / `action`。
- 回调只向其他插件的队列做非阻塞 `tryEnqueue`，不持有 dispatcher 锁等待，不影响 lane FIFO。
- Bridge 仍以第一层的入队结果判定 `DeliveryOutcomeDelivered`；后续层的结果通过 `recordOutcome` 与一条 `component=dispatch` 的 Debug 日志（`stopped_by`、`skipped_plugins`）呈现。
- `worker.go` 中 `plugin.not_handled` 从失败追踪中豁免。
- 管理面：`management` 插件摘要与详情投影新增两个字段；`web/src/views/plugins/PluginDetailView.vue` 在并发度旁展示优先级与阻断；`command_conflicts` 语义改为"同名命令由多个插件声明，按优先级与插件 ID 顺序投递"。

### 5.3 KV TTL 与存储迁移

`storage` 迁移机制（`store_schema.go`）：

```go
type migration struct{ From, To string; Statements []string }

var migrations = []migration{{
    From: "000001", To: "000002",
    Statements: []string{
        "ALTER TABLE plugin_kv ADD COLUMN expires_at_ms INTEGER",
        "CREATE INDEX IF NOT EXISTS idx_plugin_kv_expires_at ON plugin_kv (expires_at_ms) WHERE expires_at_ms IS NOT NULL",
    },
}}
```

- `initializeSchema` 遇到旧版本时沿迁移链逐步执行，每步一个事务并更新 `schema_metadata.version`；未知版本仍报错。
- `schema.sql` 更新为 `000002` 的完整结构供全新初始化；新增测试比较"全新 `schema.sql`"与"`000001` 结构 + 迁移"的 `sqlite_master` 归一化结果，防止两条路径漂移。
- `storage.IsRestorableSchemaVersion(v)` 覆盖 `000001` 与 `000002`；`recovery.EvaluateRestore` 用它替换等值比较；备份 manifest 记录归档实际版本。
- 文档：`docs/architecture/platform-runtime.md`、`docs/user/recovery.md`、`docs/release/delivery-and-upgrade.md`、`contracts/README.md` 中"只处理当前格式"的表述改为"接受可前向迁移的旧结构"。

KV 仓储（`plugins/storage/kv.go`、`sqlcqueries/pluginkv.sql`）：

```sql
-- GetKV / GetKVSize：追加 AND (expires_at_ms IS NULL OR expires_at_ms > ?)
-- GetKVTotalSize：WHERE expires_at_ms IS NULL OR expires_at_ms > ?
-- UpsertKV：SET ... , expires_at_ms = excluded.expires_at_ms
-- UpsertKVIfAbsent :execresult
INSERT INTO plugin_kv (...) VALUES (...)
ON CONFLICT(plugin_id, key) DO UPDATE SET ...
WHERE plugin_kv.expires_at_ms IS NOT NULL AND plugin_kv.expires_at_ms <= ?;
-- DeleteExpiredKV :execresult
DELETE FROM plugin_kv WHERE rowid IN (
  SELECT rowid FROM plugin_kv WHERE expires_at_ms IS NOT NULL AND expires_at_ms <= ? LIMIT ?);
```

- `KVRepository.Set` 签名扩展为 `Set(ctx, pluginID, key, value, KVSetOptions{TTL, IfNotExists}, limits) (KVSetResult{Stored bool, ExpiresAt *time.Time}, error)`；`Get` 返回 `expires_at`；手写的 `List` 追加过期过滤。
- 清扫器作为 `app` 组装的后台 goroutine，每 60 秒调用 `DeleteExpiredKV` 直到单批不足 1000 行；关闭时随 App 停止。
- `actions/storage.go` 与 `runtime/actions.go` 解析新字段并返回 `stored` / `expires_at`。

### 5.4 Go SDK

- 入站：`Event.Session *SessionRef`（`ID`、`Scope`、`Turn`、`State`）；`EventType == "session.expired"` 时同样携带。
- 动作：`Actions().SessionWait(ctx, SessionWaitRequest) (SessionWaitResult, error)`、`Actions().SessionFinish(ctx, sessionID) (bool, error)`、`Actions().KVSetWithOptions(ctx, key, value, KVSetOptions{TTL time.Duration, IfNotExists bool}) (KVSetResult, error)`；既有 `KVSet` 保持不变。
- 终态：`EventContext.SetPropagation(PropagationStop | PropagationContinue)` 在写终态帧时带上字段；`EventContext.NotHandled()` 等价于 `Fail("plugin.not_handled", ...)`。
- 阻塞式糖：`EventContext.Prompt(ctx, text string, opts PromptOptions) (*EventContext, error)`：登记等待项（`notify_on_expire=true`）、以 `SendText` 结束当前事件、释放并发信号量后挂起当前 goroutine；后续带同一 `session_id` 的事件到达时，SDK 不再调用 `Handler`，而是把新的 `*EventContext` 交给挂起的续体；`session.expired` 到达时返回 `ErrPromptTimeout`；关闭时返回 `context.Canceled`。文档强调 `Prompt` 之后原事件已关闭，后续动作与终态必须使用返回的新上下文。
- 示例插件 `examples/plugins/example-session-prompt`：命令 `/pick` 列出选项并等待序号；演示登记式与阻塞式两种写法、`state` 回显、超时处理与 `if_not_exists` 会话锁。

## 六、执行清单

| ID | 工作项 | 依赖 | 状态 | 完成情况 | 验证证据 |
| --- | --- | --- | --- | --- | --- |
| A1 | manifest 契约：`priority` / `block` 与 fixtures | — | ⬜ 待处理 | | `validate_contracts.py --mode=strict` |
| A2 | 协议契约：`propagation`、`session.wait` / `session.finish`、`payload.session`、`session.expired` 与 fixtures | — | ⬜ 待处理 | | 同上；`generate-plugin-wire.py` 与 `generate-runtime-schemas.mjs --verify` |
| A3 | 协议契约：`storage.kv set` 的 `ttl_seconds` / `if_not_exists` 与 fixtures | — | ⬜ 待处理 | | 同上 |
| A4 | 配置契约：`runtime.session_wait_*` 三键、内嵌默认值、`docs/user/configuration.md` | — | ⬜ 待处理 | | `go test ./internal/config/...`；契约 strict |
| A5 | backup manifest `000002`、Web API / WebSocket 插件字段、`info.version`、生成物 | A1 | ⬜ 待处理 | | 契约 strict；`pnpm run generate:types` 无 diff 之外的产物；`generate-launcher-api.py --verify` |
| B1 | 存储前向迁移机制与恢复兼容（`000001 → 000002`） | A5 | ⬜ 待处理 | | 新增迁移测试与结构等价测试；`go test ./internal/storage/... ./internal/operations/...`；`check-server-structure.py` |
| B2 | `plugin_kv` TTL 列、sqlc 查询、仓储、清扫器、action 解析与处理 | B1, A3 | ⬜ 待处理 | | `sqlc generate && sqlc diff`；仓储单测覆盖过期读、NX、配额排除、清扫；`go test ./internal/plugins/...` |
| B3 | SDK KV 选项与协议文档 | B2 | ⬜ 待处理 | | `(cd sdk/go && GOWORK=off go test ./...)` |
| C1 | manifest 解析、Snapshot、runtime Spec、`SwapPlugin` 参数 | A1 | ⬜ 待处理 | | `go test ./internal/plugins/... ./internal/bot/pipeline/dispatch/...` |
| C2 | Dispatcher 分层链式投递、`not_handled` 豁免、观测字段 | C1 | ⬜ 待处理 | | 新增分层顺序、stop / continue、超时视为继续、单层零回归测试；`go test -race ./internal/bot/pipeline/dispatch/...` |
| C3 | runtime 终态帧 `propagation` 解析进 `Delivery` | A2, C2 | ⬜ 待处理 | | `go test ./internal/plugins/runtime/...` |
| C4 | SDK 传播控制、管理面与 Web 展示、架构与插件文档 | C3 | ⬜ 待处理 | | SDK 测试；`web` typecheck + unit；`check-doc-links.py` |
| D1 | `sessionwait` 注册表（键、作用域优先、延续、上限、过期堆、清空） | A4 | ⬜ 待处理 | | 单测 + `go test -race ./internal/bot/pipeline/sessionwait/...` |
| D2 | `session.wait` / `session.finish` local action、runtime 解析、`app` 组装 | D1, A2 | ⬜ 待处理 | | `go test ./internal/plugins/... ./internal/app/...` |
| D3 | Ingress 命中路径、`chatevent.Session`、Bridge / Dispatcher 定向、菜单跳过、生命周期清空、过期通知 | D2, C2 | ⬜ 待处理 | | chatpolicy / bridge / dispatch / lifecycle 测试；`-race` |
| D4 | SDK 会话 API、阻塞式 `Prompt`、示例插件 | D3 | ⬜ 待处理 | | SDK 测试含续体、超时、关闭；示例 `raylea-plugin build-go` 三平台 |
| D5 | 文档：协议"对话会话"章节、SDK README、bot-core / message-flow / event-model、manifest 文档 | D4 | ⬜ 待处理 | | `check-doc-links.py`、`check-agent-docs.mjs` |
| E1 | 集成测试：会话往返、分层阻断、KV TTL；全量门禁 | B3, C4, D5 | ⬜ 待处理 | | `server/tests/integration` 新用例；`go build/vet/test ./...`、golangci-lint、契约 strict、SDK、Web、Launcher `--verify` |
| E2 | 发布整理：`docs/CHANGELOGS/v0.6.md`、发布说明 | E1 | ⬜ 待处理 | | 文档链接检查 |

执行顺序：A1 → A2 → A3 → A4 → A5 → B1 → C1 → C2 → C3 → B2 → B3 → C4 → D1 → D2 → D3 → D4 → D5 → E1 → E2。

- A 组先于一切：契约先行，生成物随契约提交。
- B1 先于 B2：迁移机制是结构变更的前提，也独立于业务逻辑可单独验收。
- C2 先于 D3：会话定向投递复用分层链路的"单目标、不 fan-out"路径。
- 每个工作项一个或多个 Conventional Commit，标题与正文中文，直接提交到 `main`。

## 七、验证基线

每个工作项完成时至少运行：

```text
cd server && go build ./... && go vet ./... && go test ./...
cd server && golangci-lint run --timeout=10m --default=none -E errcheck -E staticcheck -E govet -E unused -E ineffassign ./...
python scripts/ci/validate_contracts.py --mode=strict
python scripts/generate-plugin-wire.py --verify && node scripts/generate-runtime-schemas.mjs --verify
(cd sdk/go && GOWORK=off go test ./...)
python scripts/check-doc-links.py && node scripts/check-agent-docs.mjs
```

并发相关（C2、D1、D3）追加 `go test -race`，本机无 cgo 时由 CI nightly 覆盖；存储相关追加 `sqlc generate && sqlc diff` 与 `python scripts/check-server-structure.py`；Web 相关追加 `pnpm run typecheck && pnpm test`。

## 八、风险与边界

| 风险 | 处理 |
| --- | --- |
| 分层投递引入回调后 lane 死锁或顺序破坏 | 回调只做非阻塞入队，不等待、不持锁；单层路径无回调；用 `-race` 与顺序测试锁定 |
| 高优先级插件超时拖慢低优先级 | 超时上限即 `plugin_event_timeout_seconds`；超时视为继续；文档提示阻断插件应快速终态 |
| 冷却或内置菜单吞掉会话回复 | 命中路径跳过命令策略与菜单，只保留超管旁路与黑名单 |
| 插件长期独占用户消息 | 超时上限、每插件活跃上限、插件停止即清空；日志可追溯 |
| 一次性等待项被消费后到重新登记之间漏消息 | 文档写明；需要连续多轮的场景使用 `max_turns > 1` |
| 数据库结构升级导致现有安装无法启动或旧备份无法恢复 | 前向迁移在启动时执行；恢复接受可迁移版本；结构等价测试防止两条路径漂移 |
| 过期行占用配额 | 配额与读取都排除过期行；清扫器限批、周期执行 |
| 旧 SDK 插件收到未知字段 | 新字段只发给登记者；`propagation` 与 `session.*` 由插件主动发出；`min_core_version` 约束 |
| 协议命名与既有"事件会话"混淆 | 文档统一"对话会话"术语并在协议文档首次出现处说明 |

## 九、验收重点

- 收到 `/pick` 后回复 "1"，登记插件收到带 `payload.session` 的 `message.*` 事件，其他插件与内置菜单未被触发，冷却计数未变化。
- 等待超时后登记了通知的插件收到 `session.expired`，未登记通知的插件什么也不收到；插件停止后其等待项立即清空。
- 两个插件声明同一命令且优先级不同时，高优先级先收到；它返回 `propagation: stop` 或声明 `block` 时低优先级不再收到；返回 `plugin.not_handled` 时低优先级照常收到且日志无 Warn。
- 全部插件保持默认优先级与 `block=false` 时，投递顺序、并发与观测计数与 v0.5 一致。
- 带 `ttl_seconds` 的键到期后 `get` 返回 `exists=false`、`list` 不再列出、配额随之释放；`if_not_exists` 在键存活期间返回 `stored=false`，到期后可再次写入。
- v0.5 安装的数据库升级后自动迁移到 `000002`；v0.5 备份在 v0.6 恢复成功并完成迁移；全新初始化与迁移得到的结构一致。
- 契约、fixtures、生成物、SDK、示例与文档只有一套字段语义。

## 十、待维护者确认的默认值

以下选择已在本计划中定案，若维护者偏好不同，改动只影响契约默认值与文档：

1. 优先级方向：数值大者先（AstrBot 风格），默认 0。
2. manifest `block` 默认 `false`，保持现有 fan-out。
3. 会话回复默认独占，且 `user` 作用域优先于 `conversation`。
4. `max_turns` 默认 1。
5. 引入数据库前向迁移机制作为 KV TTL 的前置；若不接受，B 组整体延后，A3 / B2 / B3 移出本版本。
