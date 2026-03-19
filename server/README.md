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
  - plugin 仅可返回 `result` 或 `error`
- bridge 只保留内存计数和最近事件摘要，不新增外部 API。
- 当首个可投递事件到达且当前尚无运行中的 runtime 时：
  - 按 `plugin_id` 排序选择首个 manifest 有效的单个 plugin
  - 使用事件中的 OneBot `self_id` 填充 `init.bot.id`
  - 以 lazy-start 方式补齐最小 `init -> init_ack -> event` 链路
- 建立最小任务状态类型和只读内存注册表骨架。
- 建立最小内部 management session/token validation shell。
- 当前仅提供 server 内部复用的签发与校验 primitive，不新增公开 login / session route。
- 已发现但无效的 manifest，以及 `plugin_id` 冲突项，会进入只读列表摘要。
- 这两类条目的详情查询会返回结构化错误，而不是被伪装成可运行插件。

当前命令：

- 构建：`go build ./cmd/raylea-server`
- 测试：`go test ./...`

当前 flags：

- `-config`：默认 `config/user.yaml`
- `-config-schema`：默认 `contracts/config.user.schema.json`

当前明确未实现：

- adapter 到 plugin 的事件投递。
- 插件 action 请求、send / reply / API 调用。
- 除单一 `event -> result|error` 外的 plugin protocol bridge。
- `/api/tasks`、插件安装、启用、禁用等写操作 API。
- OneBot 出站 send / reply / action API。
- OneBot 事件标准化、插件事件投递与业务处理。
- `/ws/events` 管理摘要推送。
- 公开 management session / login / launcher-token surface。
- OneBot intake observability 的持久化、重放或历史查询。
- 数据库打开、迁移执行、渲染服务、Web UI、Launcher。
- 配置默认值回填、热更新和初始化向导。
- 文件监听热刷新与目录热刷新。
- 权限授予流程执行、迁移执行与持久化 desired_state。
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
- plugin `result` 只被视为内部只读结果，不会触发任何 OneBot send / reply / action。
- plugin `error` 只被记录为内部 bridge/runtime 结果，不会升级为新的 public API。
