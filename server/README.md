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
- management HTTP / WebSocket：
  - `GET /api/config`
  - `PUT /api/config`
  - `GET /api/system/status`
  - `POST /api/system/shutdown`
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
  - richer `message.send` / `message.reply`
  - legacy compatibility `message.send_image`
  - shared outbound `message.segments` 映射到 OneBot11 消息段数组
  - `reply_to_event_id` 最近事件窗口解析与 `adapter.reply_target_missing` fallback
- runtime manager：
  - `init` / `init_progress` / `init_ack`
  - `event` / `result` / `error`
  - `ping` / `pong`
  - `shutdown`
  - crash / backoff / dead_letter
- multi-plugin runtime mainline：
  - per-plugin runtime manager
  - dispatcher fan-out
  - command-directed delivery
  - scheduler `scheduler.trigger`
  - zero-gap reload
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
- config runtime snapshot / hot reload：
  - `command.prefixes`
  - `cooldown.*`
  - `auth.super_admins`
  - `auth.default_level`
- CLI 子命令：
  - `reset-admin`
  - `backup`
  - `restore`
  - `doctor`
  - `migrate`
  - `cleanup`

## 当前边界

- richer 消息动作之外的更广插件动作族仍未进入正式链路

## 默认命令

- 构建：`go build ./cmd/raylea-server`
- 测试：`go test ./...`
