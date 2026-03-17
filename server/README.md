# Server

本目录承载 RayleaBot 的 Go 服务端工程。

Phase 4A 范围：

- 读取 `-config` 和 `-config-schema`。
- 解析 YAML 配置。
- 使用 `contracts/config.user.schema.json` 做启动前校验。
- 初始化 `slog`。
- 启动最小 HTTP 服务。
- 提供 `GET /healthz` 与 `GET /readyz`。
- 启动时扫描 `examples/plugins/` 与 `plugins/installed/`。
- 使用 `contracts/plugin-info.schema.json` 校验已发现插件的 `info.json`。
- 暴露只读插件查询：
  - `GET /api/plugins`
  - `GET /api/plugins/{plugin_id}`
- 建立最小任务状态类型和只读内存注册表骨架。
- 已发现但无效的 manifest，以及 `plugin_id` 冲突项，会进入只读列表摘要。
- 这两类条目的详情查询会返回结构化错误，而不是被伪装成可运行插件。

当前命令：

- 构建：`go build ./cmd/raylea-server`
- 测试：`go test ./...`

当前 flags：

- `-config`：默认 `config/user.yaml`
- `-config-schema`：默认 `contracts/config.user.schema.json`

当前明确未实现：

- OneBot 真实连接与 Adapter。
- 插件进程拉起、插件 IPC、plugin protocol bridge。
- `/api/tasks`、插件安装、启用、禁用等写操作 API。
- 数据库打开、迁移执行、渲染服务、Web UI、Launcher。
- 配置默认值回填、热更新和初始化向导。
- 文件监听热刷新与目录热刷新。
- 权限授予流程执行、迁移执行与持久化 desired_state。

当前插件状态边界：

- `display_state=discovered` 只表示静态发现且 manifest 校验通过。
- `display_state=invalid_manifest` 只表示静态发现但 manifest 校验失败。
- `display_state=conflict` 只表示检测到 `plugin_id` 冲突。
- 这些状态都不表示插件已经启动、授权完成或迁移完成。
- 本轮不会为冲突目录隐式选择胜者，也不会根据目录优先级覆盖已有快照。
