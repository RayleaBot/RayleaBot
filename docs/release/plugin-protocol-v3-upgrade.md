# Plugin Protocol v3 Upgrade

JSONL 插件协议升级为 v3，支持同一宿主上的多个适配器身份。manifest v3、artifact v2、管理页 bridge v3 保持原有版本；这些版本分别描述不同边界。

## 插件升级

1. 更新 Go SDK，重新构建原生插件；其他语言实现按 `contracts/plugin-protocol.schema.json` 更新。
2. 从 `init.bots` 读取身份数组，每项包含 `source_adapter`、`source_protocol`、`id`，以及可选 `nickname`。空数组表示没有已知身份。
3. 使用 `bot.identities.changed` 的 `payload.bots` 替换整个数组，删除不再出现的实例。通知来源固定为 `platform` / `adapters.internal`。
4. 主动消息指定目标 `source_adapter`；回复继续由 `reply_to_event_id` 解析来源。不同实例上的同名 ID 不视为同一账号或会话。
5. 黑白名单写动作的增删参数提供 `scope`：OneBot 全局或完整的实例 / bot 身份。旧的无作用域请求会被拒绝，结构见[插件协议说明](../plugin/protocol.md#黑白名单)。`init.super_admins` 只用于 OneBot QQ 账号，不能与 QQ 官方 openid 比较。

Go SDK 保留 `EventContext.Bot` 作为事件来源身份的便利视图，并通过 `Bots` 提供完整快照。多身份的调度、启动或其他平台内部事件没有默认 bot，插件必须选择实例。SDK 的协议版本校验会拒绝 v2 宿主；v2 SDK 也不能处理 v3 握手。

## 数据与备份

本次按全新分发初始化配置与数据库。配置格式为 `4`，SQLite 从当前 `schema.sql` 创建结构，管理员密码使用 Argon2id；具体边界见[平台运行时](../architecture/platform-runtime.md#数据初始化与本版恢复)。

本版备份清单记录 `plugin_protocol_version=3`，归档配置、SQLite 快照、插件安装包与业务数据。恢复到空目录后，核对初始化元数据、管理员登录和插件业务状态；操作见[恢复说明](../user/recovery.md)。

正式字段与错误以[插件协议](../../contracts/plugin-protocol.schema.json)、[备份清单](../../contracts/backup-manifest.schema.json)和[发布元数据](../../contracts/release-manifest.schema.json)为准。
