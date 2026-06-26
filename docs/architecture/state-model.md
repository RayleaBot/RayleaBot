# State Model

本文档说明 RayleaBot 当前已落地的核心状态机，覆盖插件状态、后台任务、OneBot11 连接状态和三方账号凭据状态。

正式枚举值以 `contracts/` 和当前实现常量为准。

## 一、插件状态

管理 HTTP、管理 WebSocket、插件管理页 bridge、`plugin.list` local action 和系统 metrics 使用统一插件状态：

| 状态 | 含义 |
| --- | --- |
| `disabled` | 插件未启用，运行时未启动 |
| `enabled` | 插件已启用，等待运行时启动或当前未运行 |
| `starting` | 插件运行时正在启动 |
| `running` | 插件运行时已完成握手并可处理事件 |
| `stopping` | 插件运行时正在停止 |
| `failed` | 插件运行时崩溃、等待自动重试或需要人工恢复 |
| `invalid` | 插件 manifest 无效或插件 ID 冲突 |

`state_diagnosis` 提供异常状态的细节：

| kind | 含义 |
| --- | --- |
| `invalid_manifest` | manifest 校验失败 |
| `plugin_id_conflict` | 多个插件目录声明相同插件 ID |
| `crashed` | 运行时异常退出 |
| `retrying` | 运行时等待受控重启 |
| `recovery_required` | 运行时超过自动恢复阈值，可通过 `POST /api/plugins/{plugin_id}/recover` 触发受控冷启动 |

`/api/system/metrics` 使用 `raylea_plugin_state{state="..."}` 统计各状态插件数量。

## 二、插件内部运行时状态

```plain
stopped -> starting -> running -> stopping -> stopped
running / starting -> crashed -> retry wait -> starting
crashed -> recovery required
```

| 状态 | 含义 |
| --- | --- |
| `stopped` | 子进程未运行或已正常退出 |
| `starting` | 子进程已启动，等待握手完成 |
| `running` | 已完成握手并处理事件 |
| `stopping` | 已发送 `shutdown`，等待子进程退出 |
| `crashed` | 子进程异常退出 |
| `backoff` | 内部等待受控重启，管理面投影为 `failed` + `retrying` |
| recovery required | 超过自动恢复阈值，管理面投影为 `failed` + `recovery_required` |

插件启用意图持久化保存。运行时状态由 per-plugin runtime manager 维护，并通过管理面投影到用户可见状态。

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

## 四、OneBot11 Adapter 聚合状态

| 状态 | 含义 |
| --- | --- |
| `idle` | 尚未发起连接 |
| `connecting` | 正在建立连接或等待 ready |
| `connected` | 连接建立并通过正式 ready 语义 |
| `auth_failed` | 鉴权失败，等待人工处理 |
| `reconnecting` | 链路中断后按 backoff 重连 |
| `stopped` | 连接流程被主动停止 |

## 五、OneBot11 协议快照状态

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

## 六、三方账号状态

| 状态 | 含义 |
| --- | --- |
| `valid` | CK 已校验可用 |
| `invalid` | CK 已失效，需要重新保存或扫码 |
| `unknown` | CK 尚未完成校验或校验结果暂不可用 |

三方账号平台：

| 平台 | 含义 |
| --- | --- |
| `bilibili` | Bilibili 账号 CK |
| `weibo` | 微博账号 CK |
| `douyin` | 抖音账号 CK |
| `netease_music` | 网易云音乐账号 CK |

平台只保存、删除、扫码和校验 CK。订阅状态、用户解析、内容检查和立即检查由订阅中心插件管理。
