# Server

本目录承载 RayleaBot 的 Go 服务端工程。

## 当前已接线能力

- `cmd/raylea-server` 入口、`-config` / `-config-schema` flags
- `config/user.yaml` 读取与 `contracts/config.user.schema.json` 校验
- `GET /healthz`、`GET /readyz`
- SQLite store、migration runner、auth persistence、task persistence、plugin desired_state persistence、grant persistence、secret store
- plugin discovery：当前扫描 `plugins/builtin`、`examples/plugins` 与 `plugins/installed`
- management auth surface：
  - `POST /api/setup/admin`
  - `GET /api/setup/status`
  - `POST /api/session/login`
  - `DELETE /api/session`
  - `POST /api/session/launcher-token`
  - `POST /api/session/launcher-admission`
- management HTTP / WebSocket：
  - `GET /api/config`
  - `PUT /api/config`
  - `GET /api/system/status`
  - `POST /api/system/shutdown`
  - `POST /api/system/render/preview`
  - `GET /api/system/render/artifacts/{artifact_id}`
  - `GET /api/logs`
  - `GET /api/tasks`
  - `GET /api/tasks/{task_id}`
  - `POST /api/tasks/{task_id}/cancel`
  - `/ws/events`
  - `/ws/tasks`
  - `/ws/logs`
  - `/ws/plugins/{id}/console`
- plugin lifecycle：
  - install / enable / disable / reload / uninstall
  - grants list / grant / revoke
  - builtin plugin 默认发现、默认启用、拒绝卸载
- OneBot11 reverse WebSocket adapter：
  - ready frame gating
  - reconnect / heartbeat timeout
  - message / notice 最小归一化
  - `message.send` / `message.reply`
  - shared outbound `message.segments` 映射到 OneBot11 消息段数组
  - `reply_to_event_id` 最近事件窗口解析与 `adapter.reply_target_missing` fallback
- runtime manager：
  - `init` / `init_progress` / `init_ack`
  - `event` / `result` / `error`
  - `ping` / `pong`
  - `shutdown`
  - local action RPC for `logger.write`、`storage.kv`、`storage.file`、`http.request`、`config.read`、`config.write`、`scheduler.create`、`event.expose_webhook`、`render.image`
  - crash / backoff / dead_letter
- multi-plugin runtime mainline：
  - per-plugin runtime manager
  - dispatcher fan-out
  - command-directed delivery
  - scheduler `scheduler.trigger`
  - zero-gap reload
- plugin local actions：
  - `logger.write` through management log redaction / persistence
  - plugin-scoped `storage.kv` persistence with SQLite-backed limits
  - `storage.file` scoped to `plugin_data` with path traversal / symlink rejection and per-plugin workdir limits
  - `http.request` scoped by `http_hosts` with DNS preflight, SSRF guards, controlled private-host exceptions, timeout, and retry policy
- live chat command policy：
  - blacklist pre-check
  - command permission enforcement
  - cooldown enforcement
  - optional cooldown reply
- temporal grants：
  - `expires_at` persistence
  - effective-grant filtering
  - enable / reload / reconcile / crash restart expiry enforcement
- management log persistence：
  - persisted summary storage
  - `/api/logs` historical queries
  - `/ws/logs` persisted replay + live stream
- launcher bootstrap companion：
  - loopback-only launcher token issuance
  - launcher admission to normal management sessions
  - Web `?token=` automatic admission flow
- config runtime snapshot / hot reload：
  - `command.prefixes`
  - `cooldown.*`
  - `auth.super_admins`
  - `auth.default_level`
  - `render.timeout_seconds`
  - `render.queue_wait_timeout_seconds`
  - `render.queue_max_length`
  - `storage.kv_*`
  - `storage.file_*`
  - `http.*`
  - `logging.retention_days`
  - `logging.rate_limit_per_plugin`
- Render Service：
  - Chromium 渲染与 bounded queue
  - `templates/` 模板注册、input schema 校验与缓存键生成
  - `render.preview` 任务流、artifact registry 与同源图片读取面
  - startup logs、`/readyz`、CLI `doctor` 与 Launcher preflight 的统一资源诊断
- CLI 子命令：
  - `reset-admin`
  - `backup`
  - `restore`
  - `doctor`
  - `migrate`
  - `cleanup`

## 当前边界

- 更广插件动作族尚未 formalize

## 默认命令

- 构建：`go build ./cmd/raylea-server`
- 测试：`go test ./...`
