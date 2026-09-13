# Acceptance and Risks

本页定义正式发布的风险控制与验收门槛。

## 主要风险

| 风险 | 控制 |
| --- | --- |
| 插件包伪造、篡改或宿主能力扩大 | manifest/artifact 校验、商店归档摘要、实际文件扫描、平台/二进制格式检查、权限展示、必要的可信代码确认和归档资源上限 |
| 插件直接网络、文件或子进程行为 | 安装和更新前明确提示完全可信本地代码；当前没有插件 OS 强沙盒，只启用来源与代码均可信的版本 |
| Chromium、FFmpeg、SQLite、聊天适配器或原生插件进程故障 | readiness、diagnostics、结构化错误、恢复摘要和受控重试 |

## 发布验收

正式发行物必须通过：

- strict contracts、valid/invalid fixtures、embedded schema 和 generated types drift；
- server tests、目标包 `-race`、server build 与 binary-mode govulncheck；
- Web 与 Launcher typecheck、test、build 和受影响的 E2E；
- Go SDK 与全部插件 race 测试、Vue SDK/页面 typecheck/test/build、三平台 artifact 构建与校验；
- 四种归档的 `LICENSE`、`THIRD_PARTY_NOTICES.md`、metadata、artifact smoke 和 recovery drill；
- doctor、agent docs、文档链接和 `git diff --check`。

插件验收以 manifest v3、JSONL protocol v3、artifact v2 和 bridge v3 为准。负向用例必须拒绝不符合这些契约的输入、错误平台、篡改摘要、错误二进制、缺失 UI 资源和非单根目录 ZIP。正式包检查必须确认不存在插件源码、源码 SDK、`node_modules` 与托管语言运行时。

## 更新检查与恢复验收

更新检查与安装覆盖版本比较、平台产物选择、网络失败、无效元数据、发布页与下载地址校验、归档大小与 CRC 校验、包内版本信息校验、替换中断后重新执行，以及 Windows 上替换运行中的程序。

恢复验收使用本版生成的 backup manifest v3，核对 `config/user.yaml`、`data/**` 和 `plugins/installed/**`；清单、配置或数据库结构版本不符时拒绝恢复。无法启动的插件保持禁用并给出人工处理建议，插件持久化数据保持完整。

## 产品验收

- OneBot11 `reverse_ws`、`forward_ws`、`http_api` 和 `webhook` 具备可观察的连接、鉴权与失败状态。
- QQ 官方机器人及同协议多实例覆盖连接、身份隔离、入站和出站路由；无适配器与仅 QQ 官方实例配置能保持真实状态。
- 首次初始化、cookie 会话、CSRF、WebSocket Origin 和 Launcher control token 的安全用例全部通过。
- 插件安装、卸载、启停、重载、自定义管理页动作、日志和任务终态可追溯。
- 任务队列满时不产生 orphan task；重启把未完成任务收敛为 interrupted。
- 插件事件、调度触发、出站消息、渲染和恢复共享正式状态与错误语义。
- Web、Launcher 和 CLI 不维护服务端状态的平行副本。
- WCAG 2.2 AA 的键盘、焦点、ARIA、对比度、reduced-motion 和目标视口验收通过。

## 范围与限制

当前范围之外的能力见[项目章程](../RayleaBot机器人项目规划.md)的非目标。插件商店消费 HTTPS 静态目录并校验资产归档 SHA-256；第三方插件是用户按来源与权限要求确认后执行的完全可信本地代码。
