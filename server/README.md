# Server

本目录承载 RayleaBot 的 Go 服务端工程。

Phase 3 范围：

- 读取 `-config` 和 `-config-schema`。
- 解析 YAML 配置。
- 使用 `contracts/config.user.schema.json` 做启动前校验。
- 初始化 `slog`。
- 启动最小 HTTP 服务。
- 提供 `GET /healthz` 与 `GET /readyz`。
- 建立最小任务状态类型和只读内存注册表骨架。

当前命令：

- 构建：`go build ./cmd/raylea-server`
- 测试：`go test ./...`

当前 flags：

- `-config`：默认 `config/user.yaml`
- `-config-schema`：默认 `contracts/config.user.schema.json`

当前明确未实现：

- OneBot 真实连接与 Adapter。
- 插件目录扫描、插件进程拉起、插件 IPC。
- `/api/tasks`、`/api/plugins`、安装 / 启停等管理 API。
- 数据库打开、迁移执行、渲染服务、Web UI、Launcher。
- 配置默认值回填、热更新和初始化向导。
