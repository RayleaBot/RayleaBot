# Server

本目录包含 RayleaBot 的 Go 服务端工程。

## 服务端能力

- 配置校验与热更新、SQLite 持久化、管理认证、secret store 与日志。
- OneBot11 和 QQ 官方适配器实例、统一聊天事件、命令治理与出站消息。
- 按适配器实例统计收到与确认发出的消息，提供小时或本地自然日趋势、前期比较及连接/服务中断时段。
- 插件 artifact 安装、商店来源、生命周期、JSONL 协议、私有存储与宿主动作。
- 调度、模板渲染、插件浏览器会话、运行资源准备、备份恢复和版本检查。
- HTTP/WebSocket 管理面、Launcher 本机控制与离线 CLI。

接口、错误码、配置和插件协议以 [`contracts/`](../contracts/README.md) 为准；完整 HTTP 操作见 [OpenAPI](../contracts/web-api.openapi.yaml)。使用说明见[管理面职责](../docs/user/management-surface.md)和 [CLI](../docs/user/cli.md)。

诊断检查位于 `internal/operations/diagnostics`，由 CLI 与在线导出共用；CLI 不定义在线领域服务的业务逻辑。插件启停由 lifecycle 控制器执行，HTTP handler 只校验传输和映射结果。

调度作业摘要、排序、执行时区与载荷展示由 `internal/scheduler` 构建。离线恢复的隔离预检与文件事务归 `internal/operations/recovery`，管理员凭据重置事务归 `internal/platform/auth`；CLI 负责参数、锁、输出与退出码。

聊天事件、消息段和出站消息命令归 `internal/bot/chatevent`；插件声明与执行结果归 `internal/plugins`；调度运行记录归 `internal/scheduler`。Dispatcher 通过投递接口使用运行时，适配器路由位于 `internal/bot/pipeline/outbound`。只有 lifecycle 管理进程重载和旧实例回收，JSONL 帧留在 `plugins/runtime`。

## 当前边界

消息统计由 `internal/bot/messagestats` 维护，通过 `GET /api/system/message-stats` 查询。收到计数位于公共事件入口，早于路由和治理；发送只计平台确认成功的逻辑发送。SQLite 保存 UTC 小时计数、适配器协议与最近收信时间、统计起点、服务运行记录及连接离线区间；这些统计数据不自动清理，也不从旧日志回填。当前数据库结构为 `000008`，管理日志使用 Unix 纳秒整数及普通分页与过期清理索引，插件 KV 使用配额核算和前缀列表的元数据索引；管理会话仅保存随机不透明令牌的 SHA-256 哈希。

统计增量与存活时间每 30 秒事务写入一次，查询同时读取未写入的增量。优雅关闭在现有最终持久化预算内刷盘；异常退出可能丢失最后一批未刷盘计数，停机起点按上次存活记录估计。日桶使用与配置响应相同的有效时区，按小时起点归属自然日；非整点时区不拆分小时，夏令时按实际日界聚合。

- 单实例 Server 与 SQLite；聊天连接按 `adapters` 实例管理，支持 OneBot11 与 QQ 官方协议
- 插件 runtime 通过正式 local action surface 访问平台能力
- App 负责组装、运行和关闭；事件入口、协议入口、Webhook 网关、本地动作和系统能力各自由独立服务实现
- 平台提供按插件隔离的通用浏览器会话与密钥读写动作
- 插件凭据只保存在各自插件的 secret 命名空间，平台响应只暴露必要的摘要与状态

## 默认命令

环境检查、构建与测试命令见[工程基线](../docs/engineering/baseline.md)。源码启动从本目录运行 `go run ./cmd/raylea-server`，默认读取仓库根目录的 `config/user.yaml`。
