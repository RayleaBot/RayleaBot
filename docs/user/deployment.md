# Deployment

本页说明正式安装入口、运行根目录和更新方式。

## 发行物

| 产物 | 入口 | 更新方式 |
| --- | --- | --- |
| `windows-x64-full` | `RayleaLauncher.exe` | Launcher 一键更新 |
| `linux-x64-full` | `RayleaLauncher` | Launcher 一键更新 |
| `macos-arm64-full` | `RayleaLauncher.app` | Launcher 一键更新 |
| `linux-x64-server` | `raylea-server` | 停服后执行 `raylea-server update apply` |

GitHub 自动生成的源代码压缩包不是正式运行时产物。

## 首次安装

1. 从 [GitHub Releases](https://github.com/RayleaBot/RayleaBot/releases) 下载对应平台的 artifact。
2. 解压到固定目录。该目录是安装根和默认运行根。
3. 启动 Launcher 或 server，按一次性初始化入口创建管理员。

Windows 用户从解压根目录启动 `RayleaLauncher.exe`。Launcher 需要 Microsoft Edge WebView2 Runtime；窗口未出现且系统未安装该运行库时，先按包内 `WINDOWS-RUNTIME.md` 或 [Windows Desktop Runtime](../release/windows-desktop-runtime.md) 完成安装。请勿单独移动 Launcher，完整安装根应作为一个单元保留。


## 运行根目录

解压目录同时是安装根和默认运行根，各子目录的职责见[配置说明](./configuration.md)。图片渲染也可使用配置允许的系统 Chrome、Chromium 或 Edge。

## 更新

Launcher 在后台检查更新，发现新版本后可在“关于应用”中一键更新；服务端包停服后执行 `raylea-server update apply`。更新流程、手动覆盖步骤与更新策略见 [Delivery and Upgrade](../release/delivery-and-upgrade.md)。

Linux Launcher 使用系统提供的 GTK 3 和 WebKit2GTK 4.1 动态库，完整包不内嵌这些发行版组件。启动前按包内 `LINUX-RUNTIME.md` 或 [Linux Desktop Runtime](../release/linux-desktop-runtime.md) 安装所需系统包；无桌面环境时使用 `linux-x64-server`。

恢复操作见 [Recovery](./recovery.md)。

## 本机与局域网访问

默认监听 `0.0.0.0:8080`。本机使用 `http://127.0.0.1:8080`，同一局域网设备使用 `http://<服务器内网 IP>:8080`。可通过 `server.host` 与 `server.port` 修改监听地址。

已有配置中的 `web.exposure_mode`、`web.public_origin`、`web.trusted_proxy_cidrs` 和 `web.setup_local_only` 已失效，读取时会被忽略，保存或规范化配置时会被清理。已有的 `server.host` 会保留，需要开放内网时将其设为 `0.0.0.0` 或具体内网地址。

用户负责防火墙、网络隔离、端口映射及传输安全。应用按本机和局域网直连场景提供支持，管理访问不要求公开 origin、代理配置或 HTTPS。登录会话、CSRF 和 WebSocket Origin 校验按实际请求地址工作，代理转发头不参与客户端 IP 识别。

首次初始化需要一次性初始化 token；可从运行机器取得后在内网浏览器完成初始化。

插件自定义管理页使用独立域名隔离：本机可使用自动派生的 `p-<id-hash>.plugins.localhost:<port>`；内网访问需将 `web.plugin_ui_origin_template` 对应域名解析到运行机器。该设置只影响插件自定义页面，不限制主程序启动和管理访问。

## Linux systemd / LXC

- `linux-x64-server` 包含 `systemd/rayleabot.service` 示例。
- SQLite 状态库必须位于稳定的本地可写文件系统，不建议使用语义不完整的网络文件系统。
- 容器或 LXC 应显式设置时区并确认 Chromium、字体、UID/GID 映射和数据卷权限。
- 非特权 LXC 使用 bind mount 时，应校验 `subuid`、`subgid` 和目录 owner 映射。
- ARM64 Linux 可通过 `render.browser_path` 指向宿主 Chrome、Chromium 或 Edge。

## 容器边界

仓库不提供正式 Dockerfile、Compose 文件或容器镜像。自建容器仍需遵守运行根目录、SQLite、本地备份和手动更新 规则。
