# Server

本目录承载 RayleaBot 的 Go 服务端工程。

Phase 6 范围：

- 读取 `-config` 和 `-config-schema`。
- 解析 YAML 配置。
- 使用 `contracts/config.user.schema.json` 做启动前校验。
- 初始化 `slog`。
- 启动最小 HTTP 服务。
- 提供 `GET /healthz` 与 `GET /readyz`。
- 启动时扫描 `examples/plugins/` 与 `plugins/installed/`。
- 使用 `contracts/plugin-info.schema.json` 校验已发现插件的 `info.json`。
- 暴露只读插件查询：
  - `GET /api/plugins`
  - `GET /api/plugins/{plugin_id}`
- 启动 OneBot11 反向 WebSocket 只读 adapter shell。
- 使用 `onebot.ws_url` 与既有 `onebot.*` 重连参数建立只读连接尝试。
- 通过只读接收循环维护保守 adapter 状态，并把状态映射到 `/readyz`。
- 在 adapter 内对接收到的 OneBot 帧做最小只读 intake 分类。
- 仅维护内存 observability：最近帧类别、最近心跳、是否见过心跳、累计帧数、无效帧数。
- intake observability 只复用现有日志与 readiness 相关内部状态，不新增外部 API。
- 建立最小 plugin runtime manager vertical slice。
- 仅支持从已发现且 manifest 有效的插件快照构建 runtime spec。
- 仅支持最小 plugin protocol 握手：
  - platform 发送 `init`
  - plugin 返回 `init_ack`
- 在内存中维护最小 runtime lifecycle 状态。
- 支持最小 `shutdown(stop)` 退出路径。
- 建立最小 read-only adapter -> runtime event bridge。
- 当前只支持一个内部事件形状：
  - `onebot11.message_text`
  - 映射为 plugin protocol `event`
  - `event.event_type` 保留 `message.group` / `message.private`
  - plugin 可返回 `result`、`error` 或单一 `action=message.send`
- 建立最小 outbound adapter action slice：
  - plugin runtime 仅可输出单一 `action=message.send`
  - bridge 仅将该动作映射为 OneBot11 `send_msg`
  - adapter 仅维护最小 `echo` request-response 配对
  - 当前只观察窄成功/失败结果，不扩张为广义 action 平台
- bridge 只保留内存计数和最近事件摘要，不新增外部 API。
- 暴露最小 live-only `/ws/events`：
  - 仅接受已登录 management session（`Authorization: Bearer` 头优先，`session_token` 查询参数向后兼容）
  - 仅推送 `events.received` 的 `bridge_runtime` aggregate-only 摘要
  - 不提供 replay / history / backfill
- 暴露最小 `/ws/tasks`：
  - 仅接受已登录 management session（`Authorization: Bearer` 头优先，`session_token` 查询参数向后兼容）
  - 连接建立时回放当前内存 `tasks.Registry` 中的最新 task snapshots
  - 后续推送 `tasks.updated`；当前已接入 `plugin.install` 的最小异步执行切片，不提供历史持久化或更广任务编排
- 暴露最小 `/ws/logs`：
  - 仅接受已登录 management session（`Authorization: Bearer` 头优先，`session_token` 查询参数向后兼容）
  - 连接建立时回放 bounded in-memory log summaries
  - 后续仅推送 `logs.appended` 的白名单字段，不暴露任意结构化日志 attrs
  - 当前会对已知敏感字面值做基础掩码
- 暴露最小 `/ws/plugins/{id}/console`：
  - 仅接受已登录 management session（`Authorization: Bearer` 头优先，`session_token` 查询参数向后兼容）
  - 连接建立时回放每插件 bounded in-memory ring buffer
  - 后续仅推送经 platform-side redaction + rate limiting 处理后的 runtime `stderr` / `system` console frames
  - 当前不提供历史持久化，也不暴露原始协议 `stdout`
- 建立最小 SQLite foundation：
  - 启动时按配置打开 SQLite
  - 显式启用 WAL mode
  - 分离 read / write handle
  - write handle 序列化为 `MaxOpenConns=1`
  - 设置最小 busy timeout
  - 执行显式 migration runner 与 `schema_migrations`
  - 当前首个 migration 仅建立 auth 持久化后续所需的最小表
- 将当前最小 auth/session core 持久化到 SQLite：
  - 首次 bootstrap 建立的 management credential source 会写入 `auth_bootstrap_state`
  - 当前 session signing key 与 bootstrap credential source 一并窄持久化
  - active admin sessions 会写入 `admin_sessions`
  - 服务重启后仍可复用既有 bootstrap/login `session_token` 做最小管理面 admission
- 将当前插件 `desired_state` 持久化到 SQLite：
  - `plugin_instances` 表当前仅保存 `plugin_id`、`desired_state` 与 `updated_at`
  - 启动时会在 plugin discovery 之后恢复已安装插件的 `desired_state`
  - `runtime_state` 继续保持进程内语义，不做持久化
- 当首个可投递事件到达且当前尚无运行中的 runtime 时：
  - 按 `plugin_id` 排序选择首个 manifest 有效的单个 plugin
  - 使用事件中的 OneBot `self_id` 填充 `init.bot.id`
  - 以 lazy-start 方式补齐最小 `init -> init_ack -> event` 链路
- 建立最小任务状态类型和只读内存注册表骨架。
- 建立最小内部 management session/token validation shell。
- 当前提供最小公开 management auth surface：
  - `POST /api/setup/admin`
  - `GET /api/setup/status`
  - `POST /api/session/login`
- 以上公开入口里，只有 bootstrap 与 login 会返回 opaque token
- 当前提供最小受保护 management write/query surface：
  - `DELETE /api/session`
  - `POST /api/session/launcher-token`
  - `GET /api/config`
  - `PUT /api/config`
  - `GET /api/system/status`
  - `POST /api/system/shutdown`
  - `GET /api/logs`
  - `GET /api/tasks`
  - `GET /api/tasks/{task_id}`
  - `POST /api/tasks/{task_id}/cancel`
  - `POST /api/plugins/install`
  - `POST /api/plugins/{plugin_id}/enable`
  - `POST /api/plugins/{plugin_id}/disable`
- 暴露最小 bootstrap/admin 入口：
  - `POST /api/setup/admin`
  - 仅用于首次建立 management credential source，并立即返回 `session_token`
- 暴露最小 setup status / session / system handlers：
  - `GET /api/setup/status` 返回 bootstrap 是否完成
  - `DELETE /api/session` 仅撤销当前 session
  - `POST /api/session/launcher-token` 返回单次使用、短 TTL 的 opaque launcher token
  - `GET /api/config` 返回当前生效配置的可公开快照，并对敏感字段做基础掩码
  - `PUT /api/config` 按 formal schema 校验后原子写回 `config/user.yaml`，并返回 `restart_required`
  - `GET /api/system/status` 返回最小运行态摘要
  - `POST /api/system/shutdown` 仅接受 graceful shutdown 请求
- 暴露最小 logs query 入口：
  - `GET /api/logs`
  - 当前只查询 bounded in-memory log summaries
  - 字段范围与 `/ws/logs` 一致，并复用同一 redaction 逻辑
- 暴露最小 login 入口：
  - `POST /api/session/login`
  - 仅复用 bootstrap 后的 management credential source 换取 `session_token`
- 暴露最小 task query / cancel 入口：
  - `GET /api/tasks`
  - `GET /api/tasks/{task_id}`
  - `POST /api/tasks/{task_id}/cancel`
  - 当前直接复用内存 `tasks.Registry`，并对运行中的 `plugin.install` 提供最小取消接线
- 暴露最小插件安装执行入口：
  - `POST /api/plugins/install`
  - 当前支持 `local_directory` / `local_zip` 两种本地来源
  - 安装链路会执行来源准备、manifest 校验、正式目录写入、catalog refresh 与 task progress 更新
  - 当前已支持对运行中的 `plugin.install` 任务执行最小取消
- 暴露最小插件状态写入口：
  - `POST /api/plugins/{plugin_id}/enable`
  - `POST /api/plugins/{plugin_id}/disable`
  - 当前只切换并持久化 `desired_state`，不扩展为完整 runtime supervisor
- management HTTP handlers 当前共享统一 JSON error envelope 写出路径、request_id 注入与最小 panic recovery。
- 已发现但无效的 manifest，以及 `plugin_id` 冲突项，会进入只读列表摘要。
- 这两类条目的详情查询会返回结构化错误，而不是被伪装成可运行插件。

当前命令：

- 构建：`go build ./cmd/raylea-server`
- 测试：`go test ./...`

当前 flags：

- `-config`：默认 `config/user.yaml`
- `-config-schema`：默认 `contracts/config.user.schema.json`

当前明确未实现：

- 除单一 `onebot11.message_text -> event -> action(message.send)|result|error` 外的更广 adapter 到 plugin 事件投递。
- `message.send` 之外的插件 action 请求、send / reply / API 调用。
- 除单一 `event -> action(message.send)|result|error` 外的 plugin protocol bridge。
- 通用 task executor / progress writer substrate、历史持久化与更完整任务管理 API。
- 更完整 plugin.install pipeline：依赖安装与环境准备、`plugin_packages` 元数据、install scripts 授权、远程来源与 interrupted-task recovery。
- `/api/config` 的热更新、局部重载与字段级即时生效。
- `/api/logs` 的历史持久化、日志文件检索与更广查询语义。
- `send_msg` 之外的 OneBot 出站 send / reply / action API。
- OneBot 事件标准化、插件事件投递与业务处理。
- `/ws/plugins/{id}/console` 之外的更完整调试面；当前仅支持 redacted/rate-limited `stderr` / `system` console frames，不提供历史持久化、原始协议 `stdout` 或高级过滤。
- OneBot intake observability 的持久化、重放或历史查询。
- 渲染服务、Web UI、Launcher。
- 配置默认值回填、热更新和初始化向导。
- 文件监听热刷新与目录热刷新。
- 权限授予流程执行、迁移执行与更完整插件生命周期编排。
- 多协议或多 adapter 抽象。
- runtime restart loop、通用 supervisor、热重载。
- 广义事件总线、多插件 fan-out、宽事件归一化。

当前插件状态边界：

- `display_state=discovered` 只表示静态发现且 manifest 校验通过。
- `display_state=invalid_manifest` 只表示静态发现但 manifest 校验失败。
- `display_state=conflict` 只表示检测到 `plugin_id` 冲突。
- 这些状态都不表示插件已经启动、授权完成或迁移完成。
- 本轮不会为冲突目录隐式选择胜者，也不会根据目录优先级覆盖已有快照。

当前 adapter 状态边界：

- `idle`：adapter shell 尚未开始连接。
- `connecting`：正在进行反向 WebSocket 握手或等待首个 ready frame。
- `connected`：底层链路已建立，且已看到首个 `meta.heartbeat` 或 `meta.lifecycle(enable)`。
- `auth_failed`：握手阶段明确收到 401/403，不自动重连。
- `reconnecting`：连接失败、断开或心跳超时后，正在等待下一次窄退避重连。
- `stopped`：服务关闭时 adapter 已停止。
- `/readyz` 在 adapter 未连接成功时会保守返回 `degraded`，但 `/healthz` 仍只表示进程存活。

当前 runtime 状态边界：

- `starting`：子进程已拉起，正在等待 `init_ack`。
- `running`：已完成最小 `init -> init_ack` 握手。
- `stopping`：已发送 `shutdown(stop)`，正在等待退出。
- `stopped`：未运行、已退出，或最小握手失败后已回到静止态。
- 当前 runtime 状态只用于最小内部生命周期跟踪，不会在本轮被扩展成写操作 API 或虚假的 readiness。
- 当前 runtime shell 仍假定宿主机可直接提供 `python` / `node` 命令；托管 runtime 解析与绑定不在本轮范围内。

当前 bridge 状态边界：

- 只有 `onebot11.message_text` 会被接受并转发到运行中的单个 plugin runtime。
- 该内部事件会保留 OneBot 消息方向语义，输出为 `message.group` 或 `message.private`。
- 其它 adapter 事件在本轮只会被忽略，不会进入通用 dispatch framework。
- plugin `action=message.send` 会被窄映射为单一 OneBot11 `send_msg` 请求，并使用 `echo` 观察响应。
- 其它 action 种类仍不会触发任何 OneBot send / reply / action。
- plugin `result` 仍只被视为内部只读结果，不会触发额外的 OneBot 动作扩张。
- plugin `error` 只被记录为内部 bridge/runtime 结果，不会升级为新的 public API。
