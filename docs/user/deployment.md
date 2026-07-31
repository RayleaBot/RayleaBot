# Deployment

本页说明正式安装入口、运行根目录和更新方式。

## 发行物

| 产物 | 入口 | 更新方式 |
| --- | --- | --- |
| `windows-x64-full` | `RayleaLauncher.exe` | 正式签名门槛满足时支持用户确认后的事务安装，否则 guided |
| `linux-x64-full` | `RayleaLauncher` | 签名校验与 guided update |
| `macos-arm64-full` | `RayleaLauncher.app` | 签名校验与 guided update |
| `linux-x64-server` | `raylea-server` | 签名校验与 guided/manual update |

GitHub 自动生成的源代码压缩包不是正式运行时产物。

## 首次安装

1. 从 [GitHub Releases](https://github.com/RayleaBot/RayleaBot/releases) 下载对应 artifact、`release_manifest.v2.json` 和 `release_manifest.v2.sig.json`。
2. 使用正式 CLI 或发布说明提供的校验入口验证 manifest、签名和 artifact。
3. 解压到固定目录。该目录是安装根和默认运行根。
4. 启动 Launcher 或 server，按一次性初始化入口创建管理员。

Windows 用户只需启动解压根目录的 `RayleaLauncher.exe`。请勿移动该文件或删除同级 `launcher/` 目录。

首个支持 v2 发布信任的版本必须手动安装。旧 updater 不能自动跨越新的信任边界。

## 运行根目录

运行根承载：

- `config/`：默认配置和用户配置；
- `data/`：SQLite 状态库和插件业务数据；
- `cache/`：可重建缓存；
- `logs/`：运行日志和诊断输出；
- `plugins/installed/`：已安装插件；
- `templates/`：渲染模板；
- `.deps/`：Chromium、Python、Node.js 等受控运行资源。

图片渲染也可使用配置允许的系统 Chrome、Chromium 或 Edge。

## 更新

Launcher 每 6 小时在后台检查一次更新，只显示可用版本。任何下载、停服、备份、替换和回滚都需要用户确认。

### Windows

Windows 自动安装仅适用于 `windows-x64-full`，并要求 Ed25519 manifest 验签、artifact 校验和 Launcher/server/updater Authenticode 全部通过。正式证书未满足发布门槛时，Launcher 只提供 guided update，不显示安装按钮。

事务安装保留：

- `config/user.yaml`；
- `data/**`；
- `plugins/installed/**`。

安装器记录原服务状态并停服，创建 offline backup，在同卷 staging 验证新版，再以双 rename 交换安装根。新版 postflight 失败时恢复旧安装和旧状态。`rollback_failed` 表示自动恢复失败，系统保持停机并等待人工处理。

### Linux 与 macOS

Linux 和 macOS 使用 guided update：验证签名与 artifact，生成 offline backup，停止服务，替换程序文件，运行 doctor 和健康检查。失败时使用升级前包与备份恢复。

完整信任与回滚语义见 [Delivery and Upgrade](../release/delivery-and-upgrade.md)；恢复操作见 [Recovery](./recovery.md)。

## 远程管理

管理面默认只监听 loopback。远程访问应使用 HTTPS 反向代理，并配置：

- `web.exposure_mode: reverse_proxy`；
- `web.public_origin` 为唯一公开 HTTPS origin；
- `web.trusted_proxy_cidrs` 只包含实际代理地址段。

危险的 host/exposure 组合会在启动时失败。不要直接把管理端口暴露到公网。

## Linux systemd / LXC

- `linux-x64-server` 包含 `systemd/rayleabot.service` 示例。
- SQLite 状态库必须位于稳定的本地可写文件系统，不建议使用语义不完整的网络文件系统。
- 容器或 LXC 应显式设置时区并确认 Chromium、字体、UID/GID 映射和数据卷权限。
- 非特权 LXC 使用 bind mount 时，应校验 `subuid`、`subgid` 和目录 owner 映射。
- ARM64 Linux 可通过 `render.browser_path` 指向宿主 Chrome、Chromium 或 Edge。

## 容器边界

仓库不提供正式 Dockerfile、Compose 文件或容器镜像。自建容器仍需遵守运行根目录、SQLite、本地备份、签名校验和 guided update 规则。
