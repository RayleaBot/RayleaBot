# Recovery

本页说明 RayleaBot 当前正式支持的恢复、兼容检查、人工处理和运行环境准备路径。

## 当前正式恢复路径

1. 导出或准备受支持的恢复包。
2. 停止服务。
3. 执行 `restore`。
4. 重新启动服务。
5. 让平台完成兼容检查和恢复摘要生成。

当前恢复只接受 backup manifest v3。清单固定记录 `plugin_protocol_version=2`、`plugin_manifest_version=3`、`plugin_artifact_version=2` 与 `plugin_ui_bridge_version=3`；backup manifest v2 会被拒绝，不执行隐式迁移或破坏性重置。

升级默认保留 `config/user.yaml`、`data/**` 和 `plugins/installed/**`；`config/default.yaml` 与其他发行基线文件由新版本替换。v3 备份可以记录旧 manifest v2 / artifact v1 插件包事实并恢复其设置、密钥、KV、文件和已发布数据，但这些旧包仍保持不受支持和禁用状态，必须重新安装 manifest v3 / artifact v2 包后才能运行。回退旧版本时使用升级前的仓库外备份，不直接让旧版本读取较新的状态库。

## 恢复摘要

- 恢复预检和启动后兼容检查共享 `logs/recovery-summary.json`。
- CLI、Web 管理面、Launcher 和 diagnostics 导出读取同一份恢复摘要。
- 摘要会标示当前属于 `restore`、`upgrade` 或 `rollback`，并列出跳过插件、人工处理建议、下一步和确认历史。
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
- 不兼容插件可保持禁用并等待人工处理；恢复不会为了让旧包运行而改写包内容，也不会删除其持久化数据。
- 当前正式模型不提供恢复确认撤销入口。
