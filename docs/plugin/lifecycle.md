# Plugin Lifecycle

本页说明 RayleaBot 当前插件平台的发现、安装、启停、重载、升级和卸载边界。

正式 manifest 与协议字段以 `contracts/plugin-info.schema.json` 和 `contracts/plugin-protocol.schema.json` 为准。

## 插件来源与目录

| 目录 | 角色 |
| --- | --- |
| `plugins/builtin/` | 官方内置插件，跟随发行包交付 |
| `plugins/installed/` | 用户安装插件 |

- 默认 discovery 只扫描 `plugins/builtin/` 和 `plugins/installed/`。
- `examples/plugins/` 只承担示例职责，不进入默认发现主链。
- `plugins/dev/` 不属于默认正式 discovery root。

## 当前支持的运行时

- 当前正式支持 Python 与 Node.js 插件。
- 插件发布形态覆盖平台托管运行时插件和开发调试来源插件。
- 其他语言的运行形态不纳入当前正式支持范围。

## 生命周期主线

- discovery 读取合法 manifest 后进入插件目录目录表。
- 插件启用时由 per-plugin runtime manager 启动子进程并完成 `init -> init_ack` 握手；OneBot 协议身份可用时通过 `init.bot` 或 `bot.identity.changed` 提供给插件。
- 运行中通过 `ping/pong` 保活。
- 停止时先停止接收新事件，等待活跃会话排空，再发送 `shutdown`。
- 插件崩溃后进入受控 backoff；超过阈值后进入 `dead_letter`，平台同步移除该插件已注册的 webhook 路由，等待人工干预。
- `dead_letter` 状态可通过 `POST /api/plugins/{plugin_id}/dead_letter/recover` 进入受控冷启动尝试：服务端重置 crash 计数并重新拉起 runtime；若插件 `desired_state=disabled` 则同步置回 `enabled`。
- 热重载保持正式的 start-before-stop / zero-gap reload 语义。

## 安装、升级与卸载

- 插件安装、卸载和重载统一走后台任务模型。
- 安装会校验 manifest、运行时要求、依赖边界和能力声明，再更新 catalog。
- 升级涉及新增高敏权限时，平台会进入重新确认路径，不沿用旧授权结果静默启用。
- 卸载移除插件包目录与私有运行时环境，但默认保留业务数据与配置快照，便于恢复或重新安装。

## 数据与目录边界

- 插件包目录与插件业务数据目录严格分离。
- `plugins/installed/` 承载插件包和私有运行时环境。
- `data/plugins/<plugin_id>/` 承载插件业务数据与持久化内容。
- 可重建缓存、下载中间产物和失败安装残留进入 `cache/` 或临时目录，不与业务数据混放。

## 当前边界

- 当前平台不支持插件间依赖解析。
- 多运行时并行抽象、额外语言托管链路和更复杂的自动化热重载策略不在当前正式范围内。

## 相关文档

- [Capabilities and Manifest](./capabilities-and-manifest.md)
- [Protocol](./protocol.md)
- [State Model](../architecture/state-model.md)
