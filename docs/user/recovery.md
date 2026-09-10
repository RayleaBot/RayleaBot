# Recovery

本页说明 RayleaBot 当前正式支持的恢复、兼容检查、人工处理和运行环境准备路径。

## 当前正式恢复路径

1. 导出或准备受支持的恢复包。
2. 停止服务。
3. 执行 `restore`。
4. 重新启动服务。
5. 让平台完成兼容检查和恢复摘要生成。

本版备份使用 backup manifest v3，记录当前配置格式 `4`、数据库结构 `000001`，以及插件 manifest/protocol `3`、artifact `2`、UI bridge `3`。配置、SQLite 快照、插件业务数据和安装包一起恢复；未包含数据库时清单记录 `absent`，首次启动按当前结构初始化。

`restore` 可写入空安装目录。先停止目标服务并保留所需备份，再执行 `raylea-server -config <目标目录>/config/user.yaml restore <备份路径>`。恢复完成后使用同一配置启动，已有管理员凭据和插件持久化数据保持一致。

恢复先在隔离目录检查全部 ZIP 条目、配置和 SQLite 快照。非法路径、符号链接或目录联接、大小写冲突、损坏数据以及超过归档限额的输入会使整次恢复失败，不会静默跳过。归档限额见 [CLI 契约](../../contracts/cli-commands.yaml)。

配置中的安全自定义相对数据库路径（如 `custom/state.db`）会保留。绝对路径、Windows 盘符路径或 UNC 路径会重定位为目标目录内的 `data/rayleabot.db`，并同步写入恢复后的配置；来源数据库保持不变。含父级穿越、隐藏根目录或指向配置、插件、日志等受保护目录的相对路径不能恢复。

写入失败或取消时，恢复流程倒序还原已替换的文件，包括数据库旁的 WAL 文件和原恢复摘要。回滚失败会返回非零退出码，并在日志中给出保留的 `.restore-*` 工作目录；原文件按目标相对路径保存在其 `previous/` 下。此时先检查保留文件和失败原因，再继续处理，不要删除工作目录。若数据已经提交、仅清理失败，日志会明确标记 `committed=true`。

## 恢复摘要

- 恢复预检和启动后兼容检查共享 `logs/recovery-summary.json`。
- CLI、Web 管理面、Launcher 和 diagnostics 导出读取同一份恢复摘要。
- 摘要的操作类型为 `restore`，并列出跳过插件、人工处理建议、下一步和确认历史。
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
