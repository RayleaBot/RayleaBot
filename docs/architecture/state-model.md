# State Model

本文档说明 RayleaBot 当前已落地的核心状态机，覆盖插件 runtime 状态、任务状态、插件注册状态与 desired state。

正式枚举值以 `contracts/`（`websocket-events.yaml`、`web-api.openapi.yaml`）和 `server/internal/runtime/types.go`、`server/internal/tasks/tasks.go` 中的常量为准。

---

## 插件 Runtime 状态

每个已启动插件的运行时由 per-plugin `runtime.Manager` 管理，其状态机如下：

```
stopped
  ↓  启动子进程
starting
  ↓  收到 init_ack
running
  ↓  主动停止 / 服务关闭
stopping
  ↓  子进程退出
stopped

running / starting
  ↓  子进程意外退出
crashed
  ↓  未超过 crash 阈值
backoff
  ↓  等待 backoff 超时后重启
starting
  ↓  超过 crash 阈值
dead_letter
```

| 状态 | 含义 |
|------|------|
| `stopped` | 子进程未运行或已正常退出 |
| `starting` | 子进程已启动，等待 `init_ack` |
| `running` | 已完成握手，正常处理事件 |
| `stopping` | 已发送 `shutdown`，等待子进程退出 |
| `crashed` | 子进程意外退出 |
| `backoff` | 等待重启冷却，计划在 backoff 时间后恢复 `starting` |
| `dead_letter` | 崩溃次数超过阈值，放弃自动重启；需要人工干预 |

### 状态在 WebSocket 的投影

管理面 `/ws/events` 持续推送插件 runtime 状态变更，payload 形如：

```json
{
  "type": "events.received",
  "payload": {
    "kind": "plugin_state",
    "plugin_id": "hello-python",
    "runtime_state": "running",
    "desired_state": "enabled"
  }
}
```

---

## 插件注册状态与 Desired State

| 字段 | 枚举值 | 含义 |
|------|--------|------|
| `registration_state` | `installed` / `removed` | 插件包是否存在于 `plugins/installed/` 或 discovery root |
| `desired_state` | `enabled` / `disabled` | 用户期望插件运行（enabled）还是不运行（disabled） |

`desired_state` 持久化在 `plugin_instances` 表。runtime 状态由 runtime manager 维护在内存，通过 WebSocket 投影到管理面。

---

## 任务状态

所有后台任务（11 种类型）共享同一状态机：

```
pending
  ↓  Executor 开始执行
running
  ↓  成功完成          ↓  执行失败          ↓  调用方取消
succeeded            failed              cancelled
```

服务重启若任务处于 `running`，则在 Hydrate 时修正为 `interrupted`：

```
running (服务重启)
  ↓
interrupted
```

| 状态 | 含义 |
|------|------|
| `pending` | 已入队，等待执行 |
| `running` | 执行器正在处理 |
| `succeeded` | 已完成，`result_json` 含结果详情 |
| `failed` | 执行失败，`error_json` 含错误信息 |
| `cancelled` | 被调用方主动取消（`POST /api/tasks/{id}/cancel`） |
| `interrupted` | 服务重启中断，保留历史快照，不自动重试 |

### 任务类型

| 类型 | 触发入口 |
|------|----------|
| `plugin.install` | `POST /api/plugins/install` |
| `plugin.uninstall` | `POST /api/plugins/{id}/uninstall` |
| `plugin.reload` | `POST /api/plugins/{id}/reload` |
| `backup.create` | `POST /api/system/backup` |
| `recovery.recheck` | `POST /api/system/recovery/recheck` |
| `recovery.confirm` | `POST /api/system/recovery/confirm` |
| `restore.apply` | CLI `restore` |
| `config.migrate` | 服务启动配置兼容性检查 |
| `db.migrate` | 服务启动数据库迁移 |
| `runtime.bootstrap` | `POST /api/system/runtime/bootstrap` |
| `render.preview` | `POST /api/system/render/preview` |

---

## Grant 时效状态

插件 capability grant 支持可选 `expires_at` 字段：

| 条件 | 生效语义 |
|------|----------|
| `expires_at` 为 null | 永久有效 |
| `expires_at` 未来时间 | 在该时间前有效；runtime reconcile / enable / reload / crash 重启均重新过滤时效窗口 |
| `expires_at` 过去时间 | 已过期，不生效；reconcile 时不投影到 runtime capability 列表 |

---

## 连接状态

OneBot11 adapter 连接状态通过 `/ws/events` 的 `connection_status` 事件推送：

| 状态 | 含义 |
|------|------|
| `connected` | WebSocket 连接已建立，ready 握手完成 |
| `disconnected` | 连接断开，backoff 重连中 |
| `auth_failed` | 鉴权失败，不自动重连（需要检查配置） |
