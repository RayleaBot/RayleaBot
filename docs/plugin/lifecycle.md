# Plugin Lifecycle

本页说明 RayleaBot 当前插件平台的发现、安装、启停、重载、升级和卸载边界。

正式 manifest、artifact 与协议字段以 `contracts/plugin-info.schema.json`、`contracts/plugin-artifact.schema.json` 和 `contracts/plugin-protocol.schema.json` 为准。

## 插件来源与目录

- RayleaBot 没有内置插件；默认 discovery 只扫描 `plugins/installed/`。
- 官方、社区和开发插件都通过统一安装事务进入 `plugins/installed/<plugin_id>/`，运行目录本身不表达信任等级。
- `examples/plugins/` 只承担 SDK 示例职责，不进入发现、商店或发布主链。
- 开发仓库位于主仓库之外，通过 `plugin-workspace.local.json` 构建并同步，不作为源码 discovery root。
- 官方身份只来自已验证商店目录及持久化 package metadata，manifest 不能声明角色。

## 当前支持的运行时

- 插件后端是目标平台的预编译原生可执行文件，manifest 与 artifact 不声明实现语言。
- 服务端不编译插件源码、不安装语言依赖，也不准备插件语言运行时；Go SDK 与构建器是一等开发工具，但不是运行时约束。
- 核心在启动插件前准备共享 FFmpeg 资源，并向插件进程注入 `RAYLEABOT_FFMPEG_PATH` 与 `RAYLEABOT_FFPROBE_PATH`；这些绝对路径指向当前平台已校验的托管入口，不属于插件包内容。
- 插件包按 `windows-x64`、`linux-x64`、`macos-arm64` 分发；目标平台只由 `artifact.json.target_platform` 声明。
- JSONL 插件协议使用语言无关的 v2。

## 生命周期主线

- discovery 只读取已安装且通过 artifact 校验的 manifest。
- 插件启用时由 per-plugin runtime manager 启动子进程并完成 `init -> init_ack` 握手；OneBot 协议身份可用时通过 `init.bot` 或 `bot.identity.changed` 提供给插件。
- 运行中通过 `ping/pong` 保活。
- 停止时先停止接收新事件，等待活跃会话排空，再发送 `shutdown`。
- 插件刚异常退出时投影为 `state=failed`、`state_diagnosis.kind=crashed`；进入退避等待后投影为 `state=failed`、`state_diagnosis.kind=retrying`；超过重试阈值进入 dead-letter 时投影为 `state=failed`、`state_diagnosis.kind=recovery_required`。进入需人工恢复状态后，平台同步移除该插件已注册的 webhook 路由。
- `POST /api/plugins/{plugin_id}/recover` 触发受控冷启动尝试：服务端重置 crash 计数并重新拉起 runtime。
- 热重载保持正式的 start-before-stop / zero-gap reload 语义。

## 安装、升级与卸载

- 插件安装、卸载和重载统一走后台任务模型。
- 安装只接受单根目录 ZIP 或已经构建好的 artifact 目录。安装器先校验 manifest v3、artifact v2、最低 Core 版本、文件全集、大小、SHA-256、平台、后端二进制格式、Unix executable bit 和 UI 入口，再原子替换目标目录。
- 商店安装额外冻结并校验已签名目录中的归档摘要、manifest 摘要、插件 ID、版本和发布者身份；Web 必须先取得用户对本机原生代码的显式确认。
- manifest v2、artifact v1、错误平台、篡改文件、错误二进制、缺失 UI 文件及包含额外文件的包都会被拒绝。
- 升级重新执行完整 artifact 校验，并重新读取 permissions。替换失败时恢复旧包、旧 package metadata、旧模板和原 desired state。
- 卸载移除插件包目录；插件业务数据按卸载接口的正式选项处理，不存在私有语言运行环境。

## 数据与目录边界

- 插件包目录与插件业务数据目录严格分离。
- `plugins/installed/` 只承载经验证的编译产物。
- `data/plugins/<plugin_id>/` 承载插件业务数据与持久化内容。
- 可重建缓存、下载中间产物和失败安装残留进入 `cache/` 或临时目录，不与业务数据混放。

## 当前边界

- 当前平台不支持插件间依赖解析。
- 源码插件、安装脚本、托管语言运行时和旧合同兼容执行不在正式范围内。
- 旧 manifest v2 / artifact v1 包可以随 backup manifest v3 保留并恢复，但保持禁用；设置、密钥、KV、文件和已发布数据不清除，安装当前合同包后继续使用。

## 相关文档

- [Permissions and Manifest](./permissions-and-manifest.md)
- [Protocol](./protocol.md)
- [Plugin Store and Independent Development](./store-and-development.md)
- [State Model](../architecture/state-model.md)
