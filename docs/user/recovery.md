# Recovery

本页说明 RayleaBot 当前正式支持的恢复、兼容检查、人工处理和运行环境准备路径。

## 当前正式恢复路径

1. 导出或准备受支持的恢复包。
2. 停止服务。
3. 执行 `restore`。
4. 重新启动服务。
5. 让平台完成兼容检查和恢复摘要生成。

本版备份使用 backup manifest v3，记录当前配置格式 `4`、数据库结构 `000001`，以及插件 manifest/protocol `3`、artifact `2`、UI bridge `3`。配置、SQLite 快照、插件业务数据和安装包一起恢复；未包含数据库时清单记录 `absent`，首次启动按当前结构初始化。

`restore` 可写入空安装目录。停止目标服务并保留所需备份后，执行 `raylea-server -config <目标目录>/config/user.yaml restore <备份文件>`。恢复使用归档配置中的相对数据库路径；绝对来源路径会重定位为目标目录内的 `data/rayleabot.db`，并同步更新恢复后的配置。


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
- 未通过当前运行要求检查的插件保持禁用，并在恢复摘要中列出处理建议。
- 当前正式模型不提供恢复确认撤销入口。
