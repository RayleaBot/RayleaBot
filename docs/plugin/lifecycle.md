# Plugin Lifecycle

本页说明 RayleaBot 当前插件平台的发现、安装、启停、重载、升级和卸载边界。

正式 manifest、artifact 与协议字段以 `contracts/plugin-info.schema.json`、`contracts/plugin-artifact.schema.json` 和 `contracts/plugin-protocol.schema.json` 为准。

## 插件来源与目录

- RayleaBot 没有内置插件；默认 discovery 只扫描 `plugins/installed/`。
- 官方、社区和开发插件都通过统一安装事务进入 `plugins/installed/<plugin_id>/`，运行目录本身不表达信任等级。
- `examples/plugins/` 只承担 SDK 示例职责，不进入发现、商店或发布主流程。
- 开发仓库位于主仓库之外，通过 `plugin-workspace.local.json` 构建并同步，不作为源码 discovery root。
- 官方身份只来自已验证商店目录及持久化 package metadata，manifest 不能声明角色。

## 当前支持的运行时

- 插件后端是目标平台的预编译原生可执行文件，manifest 与 artifact 不声明实现语言。
- 插件在开发者环境中编译并打包；Go 插件可使用仓库提供的 SDK 和构建器。服务端运行已经构建的原生可执行文件。
- 核心在启动插件前准备共享 FFmpeg 资源，并向插件进程注入 `RAYLEABOT_FFMPEG_PATH` 与 `RAYLEABOT_FFPROBE_PATH`；这些绝对路径指向当前平台已校验的托管入口，不属于插件包内容。宿主同时注入 `RAYLEABOT_PLUGIN_DATA_DIR`，指向该插件 `data/plugins/<plugin_id>/` 的绝对路径，并在启动前创建。
- 插件包按 `windows-x64`、`linux-x64`、`macos-arm64` 分发；目标平台只由 `artifact.json.target_platform` 声明。
- JSONL 插件协议使用语言无关的 v4。

## 生命周期主线

- discovery 只读取已安装且通过 artifact 校验的 manifest。
- 插件启用时由 per-plugin runtime manager 启动子进程并完成 `init -> init_ack` 握手；通过 `init.bots` 提供按适配器实例区分的身份列表，后续 `bot.identities.changed` 替换该列表。
- 运行中通过 `ping/pong` 保活。
- 停止时先停止接收新事件，等待活跃会话排空，再发送 `shutdown`。
- 异常退出、退避重试与需人工恢复对应下文的插件状态。进入需人工恢复状态后，平台同步移除该插件已注册的 webhook 路由。
- `POST /api/plugins/{plugin_id}/recover` 触发受控冷启动尝试：服务端重置 crash 计数并重新拉起 runtime。
- 自动退避重试保留崩溃计数，握手成功不会清零；同一恢复周期内第 5 次崩溃后停止自动重试，进入 `recovery_required`。显式启停、重载、安装或人工恢复开启新的计数周期，其他插件不受影响。
- 热重载保持正式的 start-before-stop / zero-gap reload 语义。

## 插件状态

管理 HTTP、管理 WebSocket、`plugin.list` 本地动作与诊断使用同一组插件状态：

| 状态 | 含义 |
| --- | --- |
| `disabled` | 插件未启用，运行时未启动 |
| `enabled` | 插件已启用，等待运行时启动或当前未运行 |
| `starting` | 运行时正在创建或握手，可能尚无进程句柄 |
| `running` | 已完成握手并可处理事件 |
| `stopping` | 运行时正在停止，退出尚未确认 |
| `failed` | 初始化失败、运行时崩溃、等待自动重试或需要人工恢复 |
| `invalid` | manifest 无效或插件 ID 冲突 |

`state_diagnosis.kind` 细分异常：`invalid_manifest`、`plugin_id_conflict`、`initialization_failed`、`crashed`、`retrying` 与 `recovery_required`。进入 `recovery_required` 后可通过 `POST /api/plugins/{plugin_id}/recover` 触发受控冷启动。

## 安装、升级与卸载

- 插件安装、卸载和重载统一走后台任务模型。
- 安装只接受单根目录 ZIP 或已经构建好的 artifact 目录。安装器先校验 manifest v4、artifact v2、最低 Core 版本、实际文件、资源上限、平台和 UI 入口，设置 Unix 平台入口的可执行位，再原子替换目标目录。
- 商店安装额外校验目录中的归档摘要、插件 ID、版本和来源身份；首次安装或来源变化时，Web 必须取得用户对本机原生代码的显式确认。
- manifest v3 及更早版本、artifact v1、错误平台、篡改文件、错误二进制、缺失 UI 文件及包含额外文件的包都会被拒绝。
- 升级重新执行完整 artifact 校验。启用插件的新包完成初始化后，安装任务才成功；后处理失败时尝试恢复旧包、旧 package metadata、旧模板和原 desired state。
- 安装或卸载失败通过任务错误的 `operation_state` 和 `failures` 标明实际结果与失败阶段。回滚未完成时保留 `.plugin-install-*` 工作目录及其中的旧包，启动不会自动删除这些恢复材料。
- 卸载先确认插件停止，再移除包目录并清理元数据与模板。删除后的清理错误仍使任务失败，结果明确标为 `committed`；同一有效插件 ID 即使已无包目录，仍可再次提交卸载以重试清理。插件业务数据按卸载接口的正式选项处理，不存在私有语言运行环境。

宿主管理的浏览器 profile 在插件卸载时清理：先关闭所属会话，再删除该插件的 profile 目录。清理失败使卸载任务失败，保留未清理资源并允许再次卸载重试。

## 数据与目录边界

- 插件包目录与插件业务数据目录严格分离。
- `plugins/installed/` 只存放经验证的编译产物。
- `data/plugins/<plugin_id>/` 存放插件业务数据与持久化内容，由插件经 `RAYLEABOT_PLUGIN_DATA_DIR` 直接读写。
- 插件包目录经 `RAYLEABOT_PLUGIN_PACKAGE_DIR` 传入，插件进程也以它为工作目录；包内随附的文件只读，升级时整体替换。
- 可重建缓存、下载中间产物和失败安装残留进入 `cache/` 或临时目录，不与业务数据混放。

## 当前限制

- 插件包归档最大 1 GiB，最多 10000 个条目，单个文件解压后最大 64 MiB，全部解压后最大 2 GiB，压缩比最大 100；超出时返回 `plugin.package_resource_limit_exceeded`。
- 当前平台不支持插件间依赖解析。
- 源码插件、安装脚本、托管语言运行时和旧合同兼容执行不在正式范围内。
- manifest v3 及更早合同的包可以随备份保留并恢复，但显示为合同不受支持并保持禁用；设置、密钥、KV、文件和已发布数据不清除，使用新 SDK 重新构建并安装后继续使用。

## 相关文档

- [Plugin Manifest](./manifest.md)
- [Protocol](./protocol.md)
- [Plugin Store and Independent Development](./store-and-development.md)
- [Architecture Overview](../architecture/README.md)
