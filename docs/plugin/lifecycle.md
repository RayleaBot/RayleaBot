# Plugin Lifecycle

本页说明 RayleaBot 当前插件平台的发现、安装、启停、重载、升级和卸载边界。

正式 manifest、artifact 与协议字段以 `contracts/plugin-info.schema.json`、`contracts/plugin-artifact.schema.json` 和 `contracts/plugin-protocol.schema.json` 为准。

## 插件来源与目录

| 目录 | 角色 |
| --- | --- |
| `plugins/builtin/` | 官方内置插件，跟随发行包交付 |
| `plugins/installed/` | 用户安装插件 |

- 默认 discovery 只扫描 `plugins/builtin/` 和 `plugins/installed/`。
- `examples/plugins/` 只承担示例职责，不进入默认发现主链。
- `plugins/dev/` 不属于默认正式 discovery root。

## 当前支持的运行时

- 插件后端只支持 `runtime: "go"` 的平台预编译可执行文件。
- 服务端不编译插件源码、不安装语言依赖，也不准备插件语言运行时。
- 插件包按 `windows-x64`、`linux-x64`、`macos-arm64` 分发；目标平台必须同时出现在 `info.json.platforms` 和 `artifact.json.target_platform` 中。
- JSONL 插件协议继续使用语言无关的 v1。

## 生命周期主线

- discovery 只读取已安装且通过 artifact 校验的 manifest。
- 插件启用时由 per-plugin runtime manager 启动子进程并完成 `init -> init_ack` 握手；OneBot 协议身份可用时通过 `init.bot` 或 `bot.identity.changed` 提供给插件。
- 运行中通过 `ping/pong` 保活。
- 停止时先停止接收新事件，等待活跃会话排空，再发送 `shutdown`。
- 插件崩溃后进入受控重试等待；超过阈值后投影为 `state=failed` 且 `state_diagnosis.kind=recovery_required`，平台同步移除该插件已注册的 webhook 路由，等待人工干预。
- `POST /api/plugins/{plugin_id}/recover` 触发受控冷启动尝试：服务端重置 crash 计数并重新拉起 runtime。
- 热重载保持正式的 start-before-stop / zero-gap reload 语义。

## 安装、升级与卸载

- 插件安装、卸载和重载统一走后台任务模型。
- 安装只接受单根目录 ZIP 或已经构建好的 artifact 目录。安装器先校验 manifest v2、artifact v1、文件全集、大小、SHA-256、平台、后端二进制格式、Unix executable bit 和 UI 入口，再原子替换目标目录。
- manifest v1、Python/Node runtime、错误平台、篡改文件、错误二进制、缺失 UI 文件及包含额外文件的包都会被拒绝。
- 升级重新执行完整 artifact 校验，并重新读取能力声明与能力参数。
- 卸载移除插件包目录；插件业务数据按卸载接口的正式选项处理，不存在私有语言运行环境。

## 数据与目录边界

- 插件包目录与插件业务数据目录严格分离。
- `plugins/installed/` 只承载经验证的编译产物。
- `data/plugins/<plugin_id>/` 承载插件业务数据与持久化内容。
- 可重建缓存、下载中间产物和失败安装残留进入 `cache/` 或临时目录，不与业务数据混放。

## 当前边界

- 当前平台不支持插件间依赖解析。
- 源码插件、安装脚本、托管语言运行时和原位升级旧插件 epoch 不在正式范围内。
- 旧插件 epoch 的备份恢复与升级返回 `plugin.reset_required`；迁移前数据只保留在外部备份中，不进入新插件状态。

## 相关文档

- [Capabilities and Manifest](./capabilities-and-manifest.md)
- [Protocol](./protocol.md)
- [State Model](../architecture/state-model.md)
