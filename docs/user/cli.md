# CLI

本页说明 RayleaBot 当前正式提供的 CLI 子命令及其使用边界。

CLI 是本地离线恢复与运维入口，不是第二套常规在线管理面。命令统一记为
`raylea <subcommand>`；实际二进制名为 `raylea-server`。

## 当前正式命令

| 命令 | 作用 |
| --- | --- |
| `raylea config init` | 创建默认配置模板并写出规范化用户配置 |
| `raylea config normalize` | 按当前 schema 整理默认模板和用户配置 |
| `raylea config validate` | 校验配置文件，不修改文件内容 |
| `raylea plugin dev-sync --artifact <path> --source <path> --plugin-id <id>` | 把已构建的开发插件 artifact 同步进本地插件安装目录 |
| `raylea version --json` | 输出当前构建版本与更新协议版本 |
| `raylea update check --json` | 获取并验证签名发布清单，不下载或安装更新 |
| `raylea update verify --manifest <path> --signature <path> --artifact <path>` | 离线验证发布清单、签名 envelope 与 artifact |
| `raylea reset-admin` | 重置管理员凭据并重新进入初始化向导 |
| `raylea backup` | 创建恢复用备份 |
| `raylea restore <backup-path>` | 在停服窗口从指定备份包恢复配置、状态与插件目录 |
| `raylea doctor` | 检查配置与 schema、SQLite `quick_check`、deps / Chromium 元数据、退役配置键和恢复摘要 |
| `raylea cleanup` | 清理可重建缓存和临时目录 |

需要覆盖默认配置位置时，把全局参数放在子命令之前：

```text
raylea-server -config <config/user.yaml> -config-schema <config.user.schema.json> doctor
```

`-config` 默认为 `config/user.yaml`；`-config-schema` 默认为 server 内置的正式配置 schema。

## 可用性矩阵

| 命令 | 在线可用 | 停服后可用 | 说明 |
| --- | --- | --- | --- |
| `config init` | 否 | 是 | 写入配置目录；服务生命周期锁被占用时拒绝执行 |
| `config normalize` | 否 | 是 | 写入配置目录；服务生命周期锁被占用时拒绝执行 |
| `config validate` | 是 | 是 | 只读取并校验配置文件 |
| `plugin dev-sync --artifact <path> --source <path> --plugin-id <id>` | 否 | 是 | 三个参数均必填；需要数据库锁，在启动前或协调的重启窗口执行 |
| `version --json` | 是 | 是 | 输出构建版本信息 |
| `update check --json` | 是 | 否 | 需要网络访问受信发布来源 |
| `update verify --manifest <path> --signature <path> --artifact <path>` | 是 | 是 | 三个参数均必填；离线校验更新包三件套 |
| `reset-admin` | 否 | 是 | 必须在停服窗口执行 |
| `backup` | 是 | 是 | 在线可导出，强一致性场景建议停服执行 |
| `restore <backup-path>` | 否 | 是 | 备份路径必填，恢复导入必须在停服状态执行 |
| `doctor` | 是 | 是 | 可在线或停服执行 |
| `cleanup` | 是 | 是 | 只能清理可重建内容 |

## 当前职责边界

- CLI 复用正式后端逻辑和任务模型，不发明独立状态语义。
- 常规插件管理、完整日志浏览和配置编辑继续统一走 Web 管理面。
- 配置命令只维护本地配置文件，不替代 Web 管理面的在线配置编辑。
- Launcher 如需触发恢复、检查或备份能力，应优先复用 CLI 或共享后端逻辑。
- `cleanup` 不触碰状态库、插件业务数据和用户配置。

## 当前环境检查重点

- `doctor` 会检查配置文件可访问性与配置 schema 可用性、SQLite `quick_check`、`.deps/manifest.json` 当前平台项与 Chromium 元数据、退役插件运行时配置键，并在存在时附带恢复摘要。
- Windows 上还会读取 `HKLM\SYSTEM\CurrentControlSet\Control\FileSystem\LongPathsEnabled`。禁用或无法读取时返回 warning；可通过组策略或把该 DWORD 设为 `1` 启用。注册表值可能已被进程缓存，修改后可能需要重启 Windows。
- 长路径支持可避免 `plugins/installed/`、`.deps/store/` 和 `cache/downloads/` 下的深层 artifact 路径超过传统限制。
