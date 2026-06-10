# State Model

本文档说明 RayleaBot 当前已落地的核心状态机，覆盖插件运行时、插件期望状态、后台任务、授权时效、OneBot11 连接状态和 Bilibili source 状态。

正式枚举值以 `contracts/` 和当前实现常量为准。

## 一、插件 Runtime 状态

```plain
stopped -> starting -> running -> stopping -> stopped
running / starting -> crashed -> backoff -> starting
crashed -> dead_letter
```

| 状态 | 含义 |
| --- | --- |
| `stopped` | 子进程未运行或已正常退出 |
| `starting` | 子进程已启动，等待握手完成 |
| `running` | 已完成握手并处理事件 |
| `stopping` | 已发送 `shutdown`，等待子进程退出 |
| `crashed` | 子进程异常退出 |
| `backoff` | 等待受控重启 |
| `dead_letter` | 超过自动恢复阈值，等待人工处理；管理面通过 `POST /api/plugins/{plugin_id}/dead_letter/recover` 触发受控冷启动尝试 |

## 二、插件注册状态与 Desired State

| 字段 | 值 |
| --- | --- |
| `registration_state` | `installed` / `removed` |
| `desired_state` | `enabled` / `disabled` |

- `desired_state` 持久化保存。
- runtime 状态由 per-plugin runtime manager 维护，并通过管理面投影到用户可见状态。

## 三、后台任务状态

```plain
pending -> running -> succeeded
pending -> running -> failed
pending -> running -> cancelled
running -> interrupted   # 服务重启
```

| 状态 | 含义 |
| --- | --- |
| `pending` | 已入队，等待执行 |
| `running` | 正在执行 |
| `succeeded` | 已成功完成 |
| `failed` | 执行失败 |
| `cancelled` | 被受支持的取消路径中止 |
| `interrupted` | 因服务重启等原因中断 |

### 当前正式任务类型

- `plugin.install`
- `plugin.uninstall`
- `plugin.reload`
- `backup.create`
- `restore.apply`
- `recovery.recheck`
- `recovery.confirm`
- `runtime.bootstrap`

## 四、Grant 时效

| 条件 | 生效语义 |
| --- | --- |
| `expires_at` 为空 | 永久有效 |
| `expires_at` 在未来 | 在到期前有效 |
| `expires_at` 已过期 | 不投影到运行时能力列表 |

## 五、OneBot11 Adapter 聚合状态

| 状态 | 含义 |
| --- | --- |
| `idle` | 尚未发起连接 |
| `connecting` | 正在建立连接或等待 ready |
| `connected` | 连接建立并通过正式 ready 语义 |
| `auth_failed` | 鉴权失败，等待人工处理 |
| `reconnecting` | 链路中断后按 backoff 重连 |
| `stopped` | 连接流程被主动停止 |

## 六、OneBot11 协议快照状态

### readiness_status

| 状态 | 含义 |
| --- | --- |
| `ready` | 当前收发链路完整可用 |
| `degraded` | 当前只具备部分收发能力 |
| `failed` | 已配置传输链路，但当前不可用 |
| `setup_required` | 尚未配置任何正式传输链路 |

### active_transports

- `active_transports` 表示当前实际参与链路的传输集合。
- WebSocket 会话、HTTP API 和 webhook 可以同时进入该集合，不压缩成单值。

### transport_status.state

| 状态 | 含义 |
| --- | --- |
| `idle` | 该传输尚未启动 |
| `listening` | 回连入口或 webhook 入口已可接收请求 |
| `connecting` | 主动连接或 HTTP API 验证中 |
| `connected` | 该传输当前可用 |
| `auth_failed` | 最近一次鉴权失败 |
| `reconnecting` | 该传输正在按 backoff 重试 |
| `stopped` | 该传输已停止 |

## 七、Bilibili Source 状态

| 状态 | 含义 |
| --- | --- |
| `disabled` | 没有可用订阅或事件源不可用 |
| `idle` | 事件源已初始化，等待可检查的订阅或账号 |
| `connecting` | 正在建立直播连接或检查动态 |
| `connected` | 直播或动态检查处于可用状态 |
| `degraded` | 部分直播连接、动态检查或账号凭据受限 |
| `failed` | 当前直播与动态检查都不可用 |

诊断等级：

| 等级 | 含义 |
| --- | --- |
| `normal` | 当前无需要处理的问题 |
| `attention` | 需要关注，但平台会继续等待或重试 |
| `action_required` | 需要人工处理，例如重新登录 Bilibili CK |

Bilibili source 状态通过 `/api/bilibili/source/status` 查询，通过 `/ws/events` 的 `source: bilibili` 分支推送摘要。三方监控列表通过 `/api/third-party/monitors` 投影当前订阅目标、直播状态和动态快照。
