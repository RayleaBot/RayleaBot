# Server

本目录包含 RayleaBot 的 Go 服务端工程。

## 服务端能力

- 配置校验与热更新、SQLite 持久化、管理认证、secret store、日志与指标。
- OneBot11 和 QQ 官方适配器实例、统一聊天事件、命令治理与出站消息。
- 插件 artifact 安装、商店来源、生命周期、JSONL 协议、私有存储与宿主动作。
- 调度、模板渲染、插件浏览器会话、运行资源准备、备份恢复和更新编排。
- HTTP/WebSocket 管理面、Launcher 本机控制与离线 CLI。

接口、错误码、配置和插件协议以 [`contracts/`](../contracts/README.md) 为准；完整 HTTP 操作见 [OpenAPI](../contracts/web-api.openapi.yaml)。使用说明见[管理面职责](../docs/user/management-surface.md)和 [CLI](../docs/user/cli.md)。

诊断检查位于 `internal/operations/diagnostics`，由 CLI 与在线导出共用；CLI 不定义在线领域服务的业务逻辑。插件启停由 lifecycle 控制器执行，HTTP handler 只校验传输和映射结果。

调度作业摘要、排序、执行时区与载荷展示由 `internal/scheduler` 构建。离线恢复的隔离预检与文件事务归 `internal/operations/recovery`，管理员凭据重置事务归 `internal/platform/auth`；CLI 负责参数、锁、输出与退出码。

聊天事件、消息段和出站消息命令归 `internal/bot/chatevent`；插件声明与执行结果归 `internal/plugins`；调度运行记录归 `internal/scheduler`。Dispatcher 通过投递接口使用运行时，适配器路由位于 `internal/bot/pipeline/outbound`。只有 lifecycle 管理进程重载和旧实例回收，JSONL 帧留在 `plugins/runtime`。

## 当前边界

- 单实例 Server 与 SQLite；聊天连接按 `adapters` 实例管理，支持 OneBot11 与 QQ 官方协议
- 插件 runtime 通过正式 local action surface 访问平台能力
- App 负责组装、运行和关闭；事件入口、协议入口、Webhook 网关、本地动作和系统能力各自由独立服务实现
- 平台提供按插件隔离的通用浏览器会话与密钥读写动作
- 插件凭据只保存在各自插件的 secret 命名空间，平台响应只暴露必要的摘要与状态

## 默认命令

- 环境检查：从仓库根目录运行 `make doctor`
- 构建：`mkdir -p dist && go build -o "dist/raylea-server$(go env GOEXE)" ./cmd/raylea-server`
- 测试：`go test ./...`
