# Recovery

本页说明 RayleaBot 当前正式支持的恢复、兼容检查、人工处理和运行环境准备路径。

## 当前正式恢复路径

1. 导出或准备受支持的恢复包。
2. 停止服务。
3. 执行 `restore`。
4. 重新启动服务。
5. 让平台完成兼容检查和恢复摘要生成。

本版备份使用 backup manifest v3，记录当前配置格式 `4`、数据库结构 `000002`，以及插件 manifest/protocol `3`、artifact `2`、UI bridge `3`。恢复也接受数据库结构 `000001`，并在首次启动时前向迁移。配置、SQLite 快照、插件业务数据和安装包一起恢复；未包含数据库时清单记录 `absent`，首次启动按当前结构初始化。

`restore` 只写入未启动过的新目录：计划写入的配置、数据库及其 `-wal`/`-shm`/`-journal`、`data/` 与 `plugins/installed/` 下的文件或恢复摘要已存在时，恢复直接拒绝且不写入任何文件。先停止目标服务并保留所需备份，再执行 `raylea-server -config <目标目录>/config/user.yaml restore <备份路径>`。恢复完成后使用同一配置启动，已有管理员凭据和插件持久化数据保持一致。

结构迁移在事务中执行，失败时数据库保持迁移前状态。更新方式与退回旧版本的做法见 [Delivery and Upgrade](../release/delivery-and-upgrade.md)；旧核心不能直接读取 `000002` 数据库。

恢复先在隔离目录检查全部 ZIP 条目、配置和 SQLite 快照。非法路径、符号链接或目录联接、大小写冲突、损坏数据以及超过归档限额的输入会使整次恢复失败，不会静默跳过。归档限额见 [CLI 契约](../../contracts/cli-commands.yaml)。

配置中的安全自定义相对数据库路径（如 `custom/state.db`）会保留。绝对路径、Windows 盘符路径或 UNC 路径会重定位为目标目录内的 `data/rayleabot.db`，并同步写入恢复后的配置；来源数据库保持不变。含父级穿越、隐藏根目录或指向配置、插件、日志等受保护目录的相对路径不能恢复。

写入失败或取消时，恢复只删除本次写入的文件和新建的目录，不备份也不还原已有文件。删除未完成时返回非零退出码并在日志中说明，检查目标目录后重新恢复到新的空目录。若数据已经提交、仅清理失败，日志会明确标记 `committed=true`，并给出保留的 `.restore-*` 工作目录。

## 恢复摘要

- 恢复预检和启动后兼容检查共享 `logs/recovery-summary.json`。
- CLI、Web 管理面、Launcher 和 diagnostics 导出读取同一份恢复摘要。
- 摘要的操作类型为 `restore`，并列出跳过插件、人工处理建议、下一步和确认历史。
- `source_db_schema_version` 记录归档结构，`target_db_schema_version` 记录首次启动后的目标结构。
- `degraded` 状态下会保留人工处理建议；`compatible` 状态下不保留人工处理建议。

## 人工处理与确认

当前正式任务入口：

- `recovery.recheck`
- `recovery.confirm`
- `runtime.bootstrap`

管理面提供：

- 重新检查恢复状态
- 确认已审阅项
- 准备运行环境

Launcher 继续提供：

- 重新检查
- 准备运行环境
- 打开管理面处理跳过插件

## 运行环境准备

- Chromium 与 FFmpeg 资源问题通过 `runtime.bootstrap` 进入正式任务模型。
- 任务结果会返回安装包下载位置和解压位置。
- 安装包下载到 `cache/downloads/runtime/`，运行环境解压到 `.deps/store/<resource-id>/<version>/`。
- 图片渲染 Chromium 可使用已准备的 `.deps` 浏览器、系统 Chrome / Chromium / Edge，或通过 `render.browser_path` 显式指定。

## 当前限制

- 当前恢复摘要保留现有人工确认历史窗口，不额外建立独立长历史资源。
- 无法启动的插件保持禁用并列出人工处理建议，持久化数据保留。
- 当前正式模型不提供恢复确认撤销入口。
