# Server

本目录承载 RayleaBot 的 Go 服务端工程。

## 当前已接线能力

- `cmd/raylea-server` 入口、`-config` / `-config-schema` flags
- `config/user.yaml` 读取与 `contracts/config.user.schema.json` 校验
- `GET /healthz`、`GET /readyz`
- SQLite store、migration runner、auth persistence、task persistence、plugin desired_state persistence、grant persistence、secret store
- plugin discovery：当前扫描 `examples/plugins` 与 `plugins/installed`
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
- OneBot11 reverse WebSocket adapter：
  - ready frame gating
  - reconnect / heartbeat timeout
  - message / notice 最小归一化
  - `message.send`、`message.reply`、`message.send_image`
- runtime manager：
  - `init` / `init_progress` / `init_ack`
  - `event` / `result` / `error`
  - `ping` / `pong`
  - `shutdown`
  - crash / backoff / dead_letter
- CLI 子命令：
  - `reset-admin`
  - `backup`
  - `restore`
  - `doctor`
  - `migrate`
  - `cleanup`

## 当前边界

- 当前 app 只装配一个 `runtime.Manager`
- `internal/dispatch` 与 `internal/command` 已有实现和测试，但尚未接入 app 主链路
- `POST /api/plugins/{plugin_id}/reload` 当前是 stop-then-start，不是 zero-gap reload
- scheduler 已 hydrate 并启动 tick loop，但 trigger 尚未接到 plugin runtime
- 聊天侧 permission / blacklist / cooldown 仍主要是基座能力，尚未进入 live command path
- `/api/logs` 与 `/ws/logs` 仍是 bounded in-memory summaries
- `plugins/builtin/` 当前未纳入默认 discovery roots

## 默认命令

- 构建：`go build ./cmd/raylea-server`
- 测试：`go test ./...`
