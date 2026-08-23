# 项目契约与说明漂移审计（2026-08-23）

## 审计快照

- 范围：`contracts/`、`server/`、`web/`、`launcher/`、`sdk/`、`docs/`、各级 README、`AGENTS.md`、`.agents/skills/`、设计治理文件、fixtures 与生成物。
- 基线：`main`，HEAD `572e3e3f`，包含当前工作树中已有的未提交改动；本文行号以该工作树为准。
- 裁决规则：对外接口、schema、错误码、事件、CLI 和发布元数据以 `contracts/` 为正式来源；实现、文档、fixtures 和生成物必须同步。
- 结论类型：
  - 契约落后：实现已经形成稳定行为，但 contract、文档或指令仍描述其他语义。
  - 实现落后：正式 contract 和用户文案已经承诺行为，但运行时没有实现。
  - 治理落后：验证、AGENTS、skills 或设计工具无法约束当前真实工程边界。

## 严重度

| 级别 | 含义 |
| --- | --- |
| P0 | 可能造成数据丢失、破坏恢复产物或绕过关键安全与资源边界 |
| P1 | 正式配置、协议或 CLI 承诺不生效，可能导致拒绝服务、错误运维或功能失败 |
| P2 | 架构、能力、目录归属、验证流程或开发指令与当前实现不一致 |
| P3 | 元数据、设计工具兼容性或低风险文案漂移 |

## P0 / P1

### 1. 备份产物不符合正式 schema，在线备份遗漏插件业务数据

**状态：已解决（2026-08-23）。** 原始结论经真实归档复现确认。`backup-manifest` 已正式加入 `data` 标签；在线与离线入口统一使用 `server/internal/backup` 的原子归档构建器，均包含配置、SQLite 快照、非数据库 `data/**`（含 `data/plugins/**`）和已安装插件，同时排除 SQLite sidecar、锁文件和快照缓存。归档写入前与恢复读取后都使用嵌入的正式 schema 校验。`TestCreateBuildsSchemaValidArchiveWithPluginBusinessData`、`TestBackupCreatesValidArchive`、`TestRestoreExtractsArchiveContents` 和在线管理备份测试覆盖真实 ZIP、SQLite `quick_check` 与恢复 round trip。

正式备份清单在 `contracts/backup-manifest.schema.json:71-84` 将目录标签限制为 `config`、`database` 和 `plugins`。CLI 在 `server/internal/cli/backup.go:76-95` 备份非数据库 `data/**` 时写入 `label: data`，生成的 `backup-manifest.json` 不符合正式 schema。

`server/internal/cli/cli_test.go:91-194` 只把清单反序列化为 Go struct，没有使用正式 schema 校验真实 ZIP，因此现有测试无法发现该问题。

在线备份在 `server/internal/system/system_backup_http.go:57-100` 只收集配置、SQLite 相关文件和 `plugins/installed/`。插件 `storage.file` 的正式根目录由 `server/internal/app/plugin_stack.go:57-63` 设置在数据库同级的 `data/plugins/`，不会进入在线备份。

影响：

- 外部工具、Launcher 或跨版本恢复流程可能拒绝 RayleaBot 自己生成的备份包。
- 在线备份恢复后会静默丢失插件文件数据。

修正边界：

- 在 contract 中冻结 `data` 目录标签和在线备份内容语义。
- CLI 与 HTTP 复用同一套 archive/manifest builder。
- 对真实生成的在线、离线 ZIP 执行 schema 校验和 round-trip restore 测试。

### 2. CLI 停服边界、数据库路径和在线任务语义不成立

**状态：已解决（2026-08-23）。** 原始结论经命令入口和自定义数据库用例确认。`plugin dev-sync`、`reset-admin`、`backup`、`restore`、`cleanup` 统一在执行前获取配置生命周期锁；CLI `backup` 与 `restore` 的正式模型收敛为停服同步命令，在线备份继续由管理 API 的可取消 `backup.create` 任务承担。CLI 数据库路径从 `database.path` 解析，恢复时把 manifest 中的 database 条目写入当前配置的数据库目标。`cleanup` 只在停服窗口按保留期清理下载缓存、渲染缓存和失败安装临时目录，并在任一清理失败时返回非零状态。`TestOfflineCommandsRefuseWhileLifecycleLockHeld`、`TestConfiguredDatabasePathDrivesResetBackupAndDoctor` 与 `TestCleanupOnlyRemovesExpiredRecoverableEntries` 已覆盖这些边界。

`server/internal/cli/cli.go:78-85` 的私有 `resolveDatabasePath` 固定返回 `<runtime-root>/data/rayleabot.db`，没有加载 `database.path`。调用该 helper 的 `reset-admin`、`backup` 和 `doctor` 在自定义数据库路径下会操作默认数据库，而不是当前正式状态库。

`contracts/cli-commands.yaml:35-74,201-220,244-271` 将 `plugin dev-sync`、`reset-admin` 和 `restore` 定义为停服命令。实现只有 `config init/normalize` 通过 `server/internal/cli/cli.go:138-153` 获取生命周期锁；`reset-admin`、`server/internal/cli/restore.go:16-113` 和 `server/internal/cli/plugin_dev.go:42-134` 均没有锁。

`contracts/cli-commands.yaml:222-239` 将在线 `backup` 定义为可取消的 `backup.create` 异步任务。CLI 在 `server/internal/cli/cli.go:36-55` 直接同步调用 `runBackup`，并在 `server/internal/cli/backup.go:112-114` 始终写入 `offline` consistency。正式异步在线任务只存在于 HTTP 服务。

`contracts/cli-commands.yaml:309-327` 承诺在线 `cleanup` 跳过活跃安装和渲染任务，只清理过期、可重建内容。`server/internal/cli/cli.go:186-233` 不检查生命周期锁、任务状态或文件年龄，会删除全部 `.plugin-install-*` 和 `cache/downloads/*`；活跃安装本身使用同名临时目录。该命令也没有处理位于 `data/render` 的渲染缓存。

影响：

- 自定义数据库场景下管理员重置、备份和 `quick_check` 作用于错误数据库。
- 在线 restore 可覆盖运行中的配置、数据库和插件文件。
- 在线 cleanup 可破坏正在执行的插件安装。
- CLI backup 没有 contract 声明的任务取消与一致性协调。

### 3. Runtime 的四个正式资源限制没有进入运行时

**状态：已解决（2026-08-23）。** 四个字段均已进入正式装配：控制事件使用独立优先队列；local action 在创建 goroutine 前执行全插件 pending 上限与固定窗口 burst admission，超限请求返回 `platform.rate_limited`；请求 ID 历史保持有界；IPC 读写均按编码 JSON 字节数限制，超长无换行 stdout 不再触发无界增长。`TestReadProtocolLineRejectsOversizedFrameWithoutNewline`、`TestWriteJSONLineRejectsOversizedFrame`、`TestLocalActionAdmissionRejectsPendingAndBurstOverflow` 与 spec 投影测试覆盖配置到运行时链路。

`contracts/config.user.schema.json:481-499,535-540` 定义：

- `runtime.max_pending_control_events_per_plugin`
- `runtime.ipc_pending_actions_max`
- `runtime.ipc_action_burst_limit`
- `runtime.ipc_message_max_bytes`

这些字段只进入 `server/internal/config/config_types.go` 和 `canonical_typed.go`，没有进入 `pluginruntime.Options`、`Spec` 或 runtime wiring。

运行期与初始化期分别在 `server/internal/plugins/runtime/manager_start.go:158-176` 和 `manager_init.go:19-35` 直接调用 `bufio.Reader.ReadBytes('\n')`。读取过程没有大小上限，超长无换行 stdout 可以在协议校验前持续增长内存。

`server/internal/plugins/runtime/manager_sessions.go:11-20` 只维护可增长的 `localActionIDs` map 和计数；`manager_local_actions.go:19-36` 对每个合法 action 直接递增并启动 goroutine，没有 pending cap 或 burst limiter。

控制事件没有独立队列，`plugin.started`、`bot.identity.changed` 与普通事件共用 `max_pending_events_per_plugin` 的单一 channel。

影响：

- 错误插件可用超长 IPC 行造成 Server 内存压力。
- 插件可在事件超时前制造无界 local-action goroutine 和状态。
- 普通事件队列拥塞会挤占控制事件。

### 4. 每插件事件排队上限可被同 lane 并发路径绕过

**状态：已解决（2026-08-23）。** 每个插件 slot 现在对 channel 与 `pendingByLane` 共享 admission 计数，事件只有在真正开始执行时才从 queued 计数扣除；同 lane 缓冲无法释放新的 admission。控制事件使用独立计数与容量。slot 关闭和 enqueue 通过同一锁串行化，同时消除了关闭队列与并发发送之间的竞态。`TestDispatchQueueLimitIncludesSameLanePendingBuffer` 与 `TestDispatchControlQueueIsIndependentAndBounded` 覆盖并发同 lane 和控制队列边界。

`contracts/config.user.schema.json:474-479` 承诺 `max_pending_events_per_plugin` 是每插件最大排队数，超过后丢弃新事件。

入口 channel 使用该容量，但 `server/internal/eventpipeline/dispatch/worker.go:20-23` 另有无界 `pendingByLane map[string][]dispatchItem`。当插件并发度大于 1 且事件集中在同一 lane 时，worker 会持续把有界 channel 搬入该 slice，生产者随后可以继续写入 channel，总 pending 数不再受配置限制。

现有 queue-full 测试只覆盖 concurrency=1，没有覆盖同 lane 并发路径。

### 5. 反向代理可信来源配置没有进入请求信任链

**状态：已解决（2026-08-23）。** HTTP 入口已接入 `TrustedProxyResolver`。只有 `public_via_reverse_proxy` 且 TCP peer 命中 `trusted_proxy_cidrs` 时才解析 `Forwarded`、`X-Forwarded-For` 或 `X-Real-IP`；地址链从右向左剥离可信代理，首个不可信地址进入登录失败限流键。非可信 peer 提交的 forwarding header 被忽略。本机 Launcher 控制入口继续按正式安全边界拒绝所有带 forwarding header 的请求。IPv4 链、带端口 IPv6 与伪造 header 均有定向测试。

`contracts/config.user.schema.json:807-847` 定义 `public_via_reverse_proxy` 会信任由 `trusted_proxy_cidrs` 指定来源发送的 `Forwarded` 和 `X-Forwarded-*`。

`TrustedProxyCIDRs` 只在 `server/internal/config/runtime_constraints.go:20-70` 中接受非空和 CIDR 格式校验。请求层没有消费该配置：

- `server/internal/httpapi/httpapi.go:313-327` 只从 TCP `RemoteAddr` 提取客户端地址。
- 登录失败限流使用该地址，反向代理后的所有用户共用代理地址。
- `server/internal/management/core.go:182-218` 遇到任一 forwarding header 就一律判定非本机，不区分可信与非可信代理。

影响：

- `trusted_proxy_cidrs` 是无运行时作用的安全配置。
- 单个远端用户可能耗尽代理地址对应的登录失败额度，连带锁定其他管理员。
- 反代下的本机判定无法执行 contract 声明的可信代理语义。

### 6. 数据保留与默认备份一致性是无效设置

**状态：已解决并明确兼容忽略边界（2026-08-23）。** `data.download_cache_retention_days` 已由停服 `cleanup` 命令消费，按条目年龄清理下载缓存。仓库没有独立 audit-log store 或通用 raw event-record store，因此 `data.audit_logs_retention_days` 与 `data.event_records_retention_days` 保留为 schema-v3 兼容字段并正式标记 deprecated/read-only，管理面不再展示；管理日志保留由 `log.retention_days` 裁决。`backup.default_consistency` 同样作为旧配置兼容字段保留并从管理面移除：CLI backup 固定为离线同步，管理 API `backup.create` 固定为在线异步，不再承诺无法执行的跨入口默认切换。

`contracts/config.user.schema.json:583-612` 定义：

- `data.audit_logs_retention_days`
- `data.event_records_retention_days`
- `data.download_cache_retention_days`

三个字段只被解析和回显，没有运行时消费者、定时清理任务或对应行为测试。下载缓存只有 CLI cleanup 的全量删除，没有按年龄执行的保留策略。

`contracts/config.user.schema.json:867-883` 定义 `backup.default_consistency`，Web 配置页允许选择 `offline` 或 `online`。CLI 在 `server/internal/cli/backup.go:112-114` 固定为 `offline`，HTTP 在线任务在 `server/internal/system/system_backup_http.go:97-100` 固定为 `online`，没有路径读取该配置。

影响：

- 设置页承诺的数据生命周期和磁盘控制不会发生。
- 修改默认备份模式并重启后，实际备份行为不变。

### 7. 存储配额的范围和软硬语义与实现不同

**状态：已解决（2026-08-23）。** `GetKVTotalSize` 改为在同一写事务中汇总全部插件，`storage.kv_total_limit_mb` 现在是全局硬上限；跨插件额度测试已覆盖。文件存储只保留 `file_max_bytes` 单文件硬限制，`plugin_workdir_soft_limit_mb` 超限后仍完成写入，返回 `usage_bytes`、`soft_limit_bytes`、`soft_limit_exceeded` 与 `cleanup_recommended` 并记录结构化告警。配置字段的 Go 名称也同步为 `PluginWorkDirSoftLimitMB`，避免内部继续传播硬配额口径。

`storage.kv_total_limit_mb` 在 `contracts/config.user.schema.json:561-566` 和 Web 文案中表示“所有插件合计上限”。`server/internal/plugins/pluginstore/kv.go:93-110` 调用按 plugin ID 汇总的查询，`server/internal/sqlcqueries/pluginkv.sql:7-8` 明确包含 `WHERE plugin_id = ?`。实际语义是每插件各有一份配额。

`storage.plugin_workdir_soft_limit_mb` 和 `web/src/locales/zh-CN/plugins.ts:79` 表示软限制，超过后告警并给出清理建议。`server/internal/plugins/actions/storage.go:161-167,225-237` 将该值映射为硬 `TotalMaxBytes`，超限立即拒绝写入并返回 `platform.value_too_large`。

影响：

- 插件数量增长时，KV 总占用可达到配置值的 N 倍。
- 插件会在 UI 所称的告警阈值处直接功能失败。

### 8. 插件初始化和消息熔断配置描述了其他行为

**状态：已解决（2026-08-23）。** 单插件初始化使用不可被 `init_progress` 延长的 wall-clock deadline；一次 runtime reconcile 使用 `plugin_init_max_total_seconds` 的共享 context budget，耗尽后跳过剩余插件。消息限流只负责速率窗口并服从调用方 context；`message.circuit_breaker_seconds` 现在控制每目标熔断器，三次连续 adapter 发送失败后 open，冷却结束只允许一个 half-open probe，成功关闭、失败重新 open，取消的 probe 会释放占用而不会让 circuit 永久阻塞，配置热更新同步更新冷却期。初始化 deadline 与熔断 open/half-open/recovery/cancel 均有行为测试。

`runtime.plugin_init_max_total_seconds` 在 `contracts/config.user.schema.json:460-465` 中表示所有插件累计初始化预算，耗尽后跳过剩余插件。实现把同一个值复制进每个插件的 `Spec.InitMaxTotal`，每个 manager 独立启动 timer；N 个插件可各自使用完整预算。

`runtime.plugin_init_timeout_seconds` 描述单插件加载和握手的最长时间。`server/internal/plugins/runtime/manager_init.go:32-53` 在每次 `init_progress` 后重置 timer，实际语义是连续静默超时。

`message.circuit_breaker_seconds` 在 `contracts/config.user.schema.json:666-671` 中表示连续发送失败后的熔断等待。`server/internal/eventpipeline/outbound/rate_limiter.go:35-120` 将其作为限流队列最大等待时间，没有连续失败计数、open/half-open 状态或恢复窗口。

### 9. 身份不可用事件无法送达运行中插件

**状态：已解决（2026-08-23）。** 生命周期控制器会向所有运行插件投递空身份 `bot.identity.changed`，空身份事件不携带 bot target，并保留空 `self_id` 语义；去重状态能够区分“尚未发送”和“已发送空身份”。Go SDK 收到该事件后清空 `EventContext.Bot` 的缓存身份。正式 fixture 同时覆盖身份可用和不可用帧，Server 与 SDK 均有回归测试。

`contracts/README.md:80-81` 和 `docs/plugin/protocol.md:66-71` 允许 `bot.identity.changed` 携带空身份，用于通知插件当前 OneBot 身份不可用。

`server/internal/plugins/lifecycle/controller.go:820-840` 对空 bot ID 直接返回，不发送事件。`sdk/go/runtime.go:224-242` 也忽略空身份，不会清除 SDK 中已有的 bot ID。

影响：

- 运行中插件无法观察身份丢失。
- 插件事件上下文可能继续持有断线前的旧 bot 身份。

## P2

### 10. 工程基线的来源优先级违反 contract-first

**状态：已解决（2026-08-23）。** 工程基线已取消跨领域总排序，分别声明产品规划、对外 contract、工程基线和 companion 的正式裁决范围，并明确产品规划不能覆盖冻结 contract。

`docs/engineering/baseline.md:8-16` 使用：

`docs/RayleaBot机器人项目规划.md > contracts/ > fixtures/examples > code`

根 `AGENTS.md` 明确 `contracts/` 是对外边界唯一正式来源，项目规划只负责产品目标、范围、顶层架构和路线图。单一总排序会让规划文案反向覆盖冻结 contract。

基线应按领域分别列出 source of truth，不能给规划文档对所有 contract 的全局优先级。

### 11. 恢复文档把整个 config 目录描述为升级保留对象

**状态：已解决（2026-08-23）。** 恢复文档现与更新事务一致，只承诺保留 `config/user.yaml`、`data/**` 和 `plugins/installed/**`，并明确 `config/default.yaml` 与其他发行基线文件随新版本替换。

`docs/user/recovery.md:13` 声明同 epoch 升级不覆盖 `config/`、`data/` 和 `plugins/installed/`。

`docs/release/delivery-and-upgrade.md:112-120` 与 `server/internal/releaseupdate/transaction.go:481-518` 只保留 `config/user.yaml`、`data/**` 和 `plugins/installed/**`。`config/default.yaml` 等发行基线文件来自新版本。

### 12. 平台架构写错插件数据目录并遗漏 Plugin Store owner

**状态：已解决（2026-08-23）。** 平台架构已把插件 artifact 与 `data/plugins/` 业务数据分开，并在 owner 表、组件职责和代码地图中补入独立的 Plugin Store Service 与 `server/internal/pluginmarket/`。

`docs/architecture/platform-architecture.md:107-116` 把 `plugins/installed/` 描述为“插件包、每插件数据与包内自定义管理页资源”。正式生命周期文档要求 artifact/data 分离，实际插件文件服务根是 `data/plugins/`。

该文档的 owner 表、组件职责和代码地图也没有独立列出 `server/internal/pluginmarket/`。`docs/engineering/implementation-order.md:40,64` 已将 Plugin Store Service 定义为独立 owner。

### 13. Scheduler 被描述为一次性任务 owner

**状态：已解决（2026-08-23）。** Scheduler 已收敛为五段 cron 周期任务 owner；有限长异步操作明确由 Task Registry 负责 admission、状态、持久化与关闭 drain。

`docs/architecture/bot-core.md:9-24` 声明 Scheduler 负责定时触发和一次性任务。

`contracts/plugin-protocol.schema.json:2048-2075` 的 `scheduler.create` 必须提供五段 cron，`server/internal/scheduler/cron.go:10-16` 只接受周期 cron。有限长的一次性操作由 Task Registry 负责。

### 14. 管理面被描述为直接消费插件事件原始 payload

**状态：已解决（2026-08-23）。** 事件模型现限定 `event.payload.onebot` 面向具备 capability 的插件；管理面只消费脱敏观测摘要与管理日志详情，不再宣称接收 raw chat payload。

`docs/architecture/event-model.md:46-51` 声明插件和管理面都能直接读取 `event.payload.onebot`。

`contracts/websocket-events.yaml:113-123` 的管理 `events.received` 只提供摘要，不是 raw chat event broadcast；管理日志详情使用另一套脱敏结构。Web 与 Launcher 没有消费 `payload.onebot`。

### 15. 出站文档承诺不存在的失败重试

**状态：已解决（2026-08-23）。** 出站文档现描述 admission、限流、熔断、冷却和单次发送；Adapter 只在发送前按可用性选择 WebSocket 或 HTTP API，选定传输失败后返回正式错误，不承诺自动重试。

`docs/architecture/message-flow.md:70-74` 声明 Outbound 执行受控重试。

`server/internal/eventpipeline/dispatch/outbound_action.go:89-98` 每次只调用一次 `SendAction`。`server/internal/onebot11/outbound_sender.go:91-116` 在 WebSocket 可用时发送一次，WebSocket 不可用时选择 HTTP API；发送失败后没有重试循环、可重试错误集合或幂等语义。

### 16. 插件文档把供应链文件写成所有合法 artifact 的必备项

**状态：已解决（2026-08-23）。** 文档已把许可证、第三方 notices 和 SPDX SBOM 限定为正式 `sdk/go/pluginbuild` 构建器的输出保证；通用安装合法性继续由 `plugin-artifact.schema.json` 裁决。

`docs/plugin/capabilities-and-manifest.md:153-159` 声明每个平台包包含许可证、第三方 notices 和 SPDX SBOM。

`contracts/plugin-artifact.schema.json:8-15,34-39` 只要求 artifact 基本字段和至少两个文件；`fixtures/plugin-artifact/ok.minimal.json:10-27` 只有 manifest 与 backend 仍为合法 fixture。服务端 validator 也只强制唯一 backend 和 manifest。

若这些文件是正式 `pluginbuild` 的输出保证，文档应限定到正式构建器；若它们是安装门禁，必须先收紧 contract、fixture 和 validator。

### 17. README 能力索引存在遗漏与错误分类

**状态：已解决（2026-08-23）。** Server README 已补齐插件 secret 删除接口；根 README 将四种入口准确归类为 OneBot11 传输方式，不再称为多平台接入。

`server/README.md:73-78` 列出插件 secrets 的 GET/PUT，但遗漏已经冻结和实现的 `DELETE /api/plugins/{plugin_id}/secrets`。

根 `README.md:14` 把 `reverse_ws`、`forward_ws`、`http_api` 和 `webhook` 称为“多平台接入”。它们是 OneBot11 的传输方式；正式产品范围不包含非 OneBot 多协议扩展。

### 18. 配置文案与当前运行语义不一致

**状态：已解决（2026-08-23）。** `server.host` 现按三种 exposure mode 描述回环或明确私网地址，并声明拒绝 wildcard；`database.path` 明确相对运行根解析；`log.retention_days` 明确裁剪 SQLite `management_logs`。Web 中文配置说明同步更新。

| 配置 | 文案 | 当前实现 |
| --- | --- | --- |
| `server.host` | 推荐 `0.0.0.0` | 三种合法 exposure mode 的运行约束都拒绝 wildcard |
| `database.path` | 相对路径基于 `data/` | `runtimepaths.ResolveDatabasePath` 相对运行根解析 |
| `log.retention_days` | 清理日志文件 | 只 prune SQLite `management_logs` |

相关位置：

- `contracts/config.user.schema.json:47-51,98-108,636-641,1031-1037`
- `server/internal/config/runtime_constraints.go:20-70`
- `server/internal/runtimepaths/paths.go:21-35`
- `server/internal/app/platform.go:129-148`

### 19. Chromium 正式元数据仍限定为图片渲染资源

**状态：已解决（2026-08-23）。** deps schema、配置 schema、contracts 索引和 Web 中文说明均把 Chromium 定义为图片渲染与抖音扫码浏览器回落共用资源；`render.browser_path` 的共享消费语义已与 wiring 一致。

`contracts/deps-manifest.schema.json:5`、`contracts/config.user.schema.json:292-296` 和 `contracts/README.md:37-39` 仍称托管 Chromium 只供图片渲染。

`server/internal/app/integration_wiring.go:65-90` 已将托管 Chromium 注入抖音扫码浏览器回落。`server/internal/deps/README.md`、用户配置与部署文档已经使用正确语义。

### 20. error catalog 头部没有覆盖自身的正式 scope

**状态：已解决（2026-08-23）。** catalog 头部 scope 已覆盖文件实际使用的 HTTP、task、WebSocket、plugin protocol、logs、CLI 和 readiness。

`contracts/error-codes.yaml:2-4` 声明 catalog 只覆盖 HTTP、task、WebSocket 和 plugin protocol。同一文件已经存在 `logs`、`cli` 和 `readiness` applies_to 项。

### 21. Launcher AGENTS 仍描述迁移前的状态来源和打开方式

**状态：已解决（2026-08-23）。** Launcher 指令现区分服务端正式 snapshot 与本机 `preflightChecks`/`recentStderr`，说明恢复摘要的本机兜底和服务端覆盖，并把 URL fragment 中一次性 setup token 的唯一例外、即时清除与请求头提交写清。

`launcher/AGENTS.md:30-34` 声明系统状态、恢复摘要和诊断都直接消费服务端正式接口，并且 Launcher 只向 Web 传普通 URL。

当前 Launcher：

- 使用本机 preflight 和 `recentStderr` 作为正式桌面诊断输入。
- 先读取本地 `recovery-summary.json`，再由可用的服务端 payload 覆盖。
- 在 `setup_required` 时通过 URL fragment 传递一次性 `setup_token`；Web 读取后立即从地址栏清除，并放入 `X-Raylea-Setup-Token` 请求头。

指令应区分服务端正式快照、Launcher-owned 本机诊断和 setup-token fragment 的唯一安全例外。

### 22. Contracts AGENTS 的绝对规则与当前正式 contract 和严格消费行为冲突

**状态：已解决（2026-08-23）。** list 规则为 contract 明确限长的 snapshot 保留例外；unknown 策略按扩展安全与安全/状态机边界区分兼容降级或 fail closed；错误 details 仅在 contract 声明时要求固定结构。

`contracts/AGENTS.md:15` 要求所有 list API 定义 pagination、sort、filter 和 empty state。`GET /api/third-party/accounts` 是有界账号快照，没有这些输入。

`contracts/AGENTS.md:23-25` 禁止未知 enum 导致异常。Launcher 的服务端 payload validator 明确 fail-closed，并由测试锁定 unknown enum 抛错。

`contracts/AGENTS.md:30-32` 要求每个错误码定义 details 形状；新错误码与通用 `ErrorEnvelope.details` 仍允许无 details 或自由对象。

这些规则需要明确 bounded snapshot、无 details 和 fail-closed 边界的例外，或同步收紧正式 contract。

### 23. Contract generated companion 清单不完整

**状态：已解决（2026-08-23）。** companion 清单已覆盖 Web API/WebSocket 类型、Launcher API/Wails bindings、Server 嵌入 config contracts/schema bytes 与 Vue SDK contract 类型；`repo-validation` 同步使用同一矩阵。

`contracts/AGENTS.md:38-40` 只具体点名：

- `web/src/types/generated.ts`
- `launcher/src/shared/web-api.generated.ts`

真实生成链还包括：

- `web/src/types/websocket.generated.ts`
- `launcher/src/renderer/bindings/`
- `server/internal/config/contracts/*`
- `sdk/vue/src/contract.generated.ts`

清单应完整覆盖生成矩阵，或只引用 `repo-validation` 和 CI 作为唯一流程入口。

### 24. repo-validation 缺少当前文档与 contract 门禁

**状态：已解决（2026-08-23）。** skill 已加入 docs 链接与 agent docs 门禁、contract validator self-test/strict、完整生成物矩阵，并明确 Windows race 只有在 CGO 与 C 编译器可用时执行，否则记录缺口并由 Linux CI/nightly 覆盖。

`.agents/skills/repo-validation/SKILL.md:20-29` 没有 `docs/` 分支，也没有 contract validator 的 `--self-test` 与 `--mode=strict`。

实际 CI 对 docs/CI 变更同时运行：

- `node scripts/check-agent-docs.mjs`
- `python scripts/check-doc-links.py`

该 skill 对 Windows 并发路径直接要求 `go test -race`，但当前 Windows `CGO_ENABLED=0` 会返回 `go: -race requires cgo`；正式 nightly 在 Ubuntu 显式设置 `CGO_ENABLED=1`。

### 25. 根 AGENTS 的 Launcher 入口和命令索引没有覆盖 Wails 模块边界

**状态：已解决（2026-08-23）。** Launcher Read First 已补入 `launcher/go.mod` 与 Wails bindings；命令索引要求在对应子工程目录执行，并明确 Windows Bash 语法使用 `gbash`，避免把 Bash 片段直接交给 PowerShell。

根 `AGENTS.md:39` 的 Launcher Read First 没有列出 `launcher/go.mod` 和 `launcher/src/renderer/bindings/`。二者已经是独立 Go module、Wails service 与生成漂移门禁的正式边界。

根 `AGENTS.md:50-58` 给出 Bash 语法的 `cd ... &&`、`mkdir -p` 和 `$(go env GOEXE)`，但没有按 `docs/engineering/baseline.md:86-92` 标注使用 `gbash`。当前 PowerShell 5.1 不能直接执行这些命令。

### 26. editing-final-state-content 的适用范围会删除正式架构事实

**状态：已解决（2026-08-23）。** skill 现明确保留架构 owner、跨层数据流、状态迁移和正式 implementation order，只删除编辑痕迹与不属于文档职责的过程说明。

`.agents/skills/editing-final-state-content/SKILL.md:12-34` 适用于所有 docs 和 AGENTS，并要求删除内部实现顺序和数据流说明。

`docs/architecture/` 的职责就是架构、事件模型和跨层边界，`docs/engineering/implementation-order.md` 是正式实施顺序来源。该 skill 没有排除这些文档，可能诱导维护者删除读者完成架构实现所需的事实。

### 27. 工程基线默认命令漏掉 Web 与 Launcher typecheck

**状态：已解决（2026-08-23）。** 工程基线的 Web 与 Launcher 默认命令均已补入 `pnpm run typecheck`。

`docs/engineering/baseline.md:94-111` 的 Web、Launcher 默认命令没有 `pnpm run typecheck`。

`docs/engineering/quality-gates.md:12-24` 与两个 `package.json` 都把 typecheck 作为正式门禁。基线应补充该命令，或明确将 quality-gates 作为完整验证清单。

### 28. 手写 SQL 例外存在已到期项目

**状态：已解决并记录限期延期（2026-08-23）。** 结构检查器现在解析 ISO `revisit_after` 并拒绝已过期条目，复审日期已具备实际门禁。五个到期条目经复核仍属于可迁移的静态 SQL，但第三方账号需要与 secret-store 投影一起迁移，治理 CRUD 需要整组迁移，Render 三组查询需要保持同一 revision/transaction 边界；本轮不以局部 sqlc 混用扩大回归面，统一续期至 `2026-10-01`，到期后 CI 会强制再次裁决而不能无限忽略。

`docs/engineering/manual-sql-exceptions.json` 中以下项目的 `revisit_after` 为 `2026-08-01`：

- `server/internal/integrations/thirdparty/accounts.go`
- `server/internal/permission/checker.go`
- `server/internal/render/repository/load.go`
- `server/internal/render/repository/queries.go`
- `server/internal/render/repository/sql.go`

对应源码仍含手写 SQL。`scripts/check-server-structure.py` 只检查字段和文件匹配，不解析日期或阻止过期例外，复审日期没有治理作用。

## P3

### 29. Impeccable 项目记录与当前工具 schema 不兼容

**状态：已解决（2026-08-23）。** `PRODUCT.md` 已移除废弃 Register、加入 schema stamp 与 `Platform`，并仅依据现有产品说明、contracts、架构和实现证据补齐 Positioning、Operating Context、Capabilities and Constraints、Evidence on Hand 与 Product Principles；未虚构市场或用户研究结论。`DESIGN.md` 的组件区改用工具可识别的 `Components` 标题，sidecar 生成日期刷新，并由仓库 token 生成链重新同步。

只读 doctor 报告：

- `PRODUCT.md:3-5` 的 `## Register` 已废弃。
- `PRODUCT.md` 没有当前 schema stamp，缺少 Positioning、Operating Context、Evidence on Hand 和 Product Principles。
- `.impeccable/design.json:2-4` 的 `generatedAt` 早于当前 `DESIGN.md`。
- `DESIGN.md:130` 已有中文“组件规则”，但 doctor 不能将其识别为当前 components section。

仓库自己的 `node scripts/generate-design-tokens.mjs --check` 通过，因此这里属于 Impeccable doctor 与仓库自定义设计生成链的兼容性缺口，不能直接视为 token 值错误。

### 30. 插件管理页设计文案仍使用冷色主操作

**状态：已解决（2026-08-23）。** 插件管理页设计说明已使用梅紫 `brand` token 定义主操作，并保持暖色 `attention` 与 success/warning/danger 的独立语义。

`docs/design/plugin-management-surface.md:22,41` 将官方插件页主操作定义为“冷色”。`DESIGN.md` 与 `design/tokens.json` 的正式主操作是梅紫 brand token。

### 31. 低风险最终态措辞冲突

**状态：已解决（2026-08-23）。** Server 指令已改为正向最终态约束：订阅/广播统一复用泛型 Hub，race 构建统一使用共享 `RaceEnabled` 常量。

`server/AGENTS.md:29,64` 使用“不再手写订阅表”“不再复制 build tag stub”等迁移口吻，与 `editing-final-state-content` 的最终态规则冲突。语义当前正确，可直接改为正向约束。

## 验证结果（最终复核）

| 检查 | 结果 |
| --- | --- |
| `python scripts/check-toolchain.py`（Go 1.26.6、Node 26.7.0、Python 3.14.7） | 通过，全部固定工具链与 Chromium 可用 |
| `node scripts/check-agent-docs.mjs` | 通过 |
| `python scripts/check-doc-links.py` | 通过，129 个 Markdown 文件 |
| `python scripts/check-server-structure.py` | 通过，0 warnings |
| `node scripts/generate-design-tokens.mjs --check` | 通过 |
| Impeccable `doctor --json` | 通过，`findings: []` |
| `node scripts/generate-runtime-schemas.mjs --verify` | 通过 |
| `python scripts/ci/validate_contracts.py --self-test` | 通过 |
| `python scripts/ci/validate_contracts.py --mode=strict` | 通过 |
| `sqlc diff` | 通过，无漂移 |
| 手写 SQL 到期门禁负例 | 通过，`2000-01-01` 条目被拒绝 |
| Server `go build` | 通过，生成 `server/dist/raylea-server.exe` |
| Server `go test ./...` | 通过，包含 architecture、integration、services 与 WebSocket 测试 |
| Go SDK `go test ./...` | 通过 |
| Web `pnpm run typecheck`、`pnpm test`、`pnpm build` | 通过，63 个 test files、304 个 tests |
| Launcher `pnpm run typecheck`、`pnpm test`、`pnpm build` | 通过，15 个 test files、62 个 tests；第三方 Tabster source-map 缺失仅产生非阻断 warning |
| Vue SDK `pnpm run typecheck`、`pnpm test`、`pnpm build` | 通过，1 个 test file、2 个 tests |
| `git diff --check` | 通过，仅有 LF/CRLF 提示 |

除结构与生成门禁外，最终验证还直接覆盖真实 ZIP/schema/恢复、同 lane 排队、控制队列、可信代理链、超长无换行 IPC、local-action admission、自定义数据库路径、过期 cleanup、全局 KV 配额、文件软限制、初始化 deadline、熔断 half-open/cancel 和身份清空。原审计所列的字段未消费、语义相反与并发旁路不再只依赖静态门禁判断。

## 已核对且无当前漂移

- `contracts/README.md` 覆盖全部 71 个 OpenAPI method/path。
- Web 正式路由与 `docs/user/management-surface.md` 页面表一致。
- 配置 apply-policy 表与当前 schema 一致。
- 插件 capability、action、event 和 message segment 清单与 contracts 基本一致。
- 发布 artifact 矩阵、工具链版本锚点和生成设计 token 当前一致。
- 历史 `docs/CHANGELOGS/v0.1.md`、`v0.2.md` 按历史记录保留，不以当前能力判定漂移。

## 修正顺序与完成情况

1. 备份内容、manifest 标签、在线/离线 ownership、恢复目标、CLI 数据库路径和生命周期锁已统一。
2. IPC 大小、pending action、burst、控制事件、总排队上限和可信代理链已接入运行时。
3. retention、backup consistency、quota、init timeout 和 circuit breaker 已完成正式语义裁决；兼容保留字段已明确标记。
4. 真实 ZIP、同 lane 并发、可信/非可信代理、超长无换行 IPC、过期 cleanup 和自定义数据库路径已有行为测试。
5. README、架构文档、AGENTS、skills、fixtures、生成物和验证门禁已同步。

## 验证边界

- Windows 当前 `CGO_ENABLED=0` 且无可用 GCC，本轮未运行 `go test -race`；并发路径由定向测试覆盖，race 仍由 Ubuntu CI/nightly 承担。
- 未验证独立插件仓库。
- 未验证线上 GitHub Release、正式 Authenticode 证书或 packaged E2E。
- 可信代理使用单元与 HTTP middleware 测试验证，未搭建真实 Nginx/Traefik 多跳环境。
- strict validator 主要证明 schema 与 fixture 结构一致，不能证明运行时消费了每个字段。

## 最终全文复核

- 全文 31 个编号问题均有当前状态：29 项完成实现或文档/治理修复；第 6 项的三个旧配置键按 schema-v3 兼容边界保留并明确忽略运行语义；第 28 项的五组 sqlc 迁移按同一 repository/transaction 边界限期延至 `2026-10-01`，到期门禁已经生效。
- 没有未处理或无解释忽略的审计项。每项状态段描述当前工作树，后续原始证据段保留初始审计快照和当时行号，不作为当前行为说明。
- 最终复核再次交叉检查 contract、实现、测试、fixtures、生成物、README、架构文档、AGENTS、skills、PRODUCT/DESIGN 与本节验证结果，未发现新的审计结论冲突。
