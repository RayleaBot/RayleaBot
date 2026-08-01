# Acceptance and Risks

本页定义正式发布的风险控制与验收门槛。

## 主要风险

| 风险 | 控制 |
| --- | --- |
| 发布元数据伪造、过期或重放 | 固定仓库与 Ed25519 公钥、双签轮换、过期时间、最高版本和同版本 digest 记录 |
| Windows artifact 被替换或 signer 不符 | manifest 摘要与 Authenticode 双重校验，正式证书和 RFC3161 timestamp |
| 安装中断、磁盘不足或新版不可用 | 外置 updater、磁盘预检、journal、同卷 staging、双 rename、postflight 与自动回滚 |
| 回滚状态不可读 | offline backup、旧安装根保留、旧版 restore/doctor 和 `rollback_failed` 停机保护 |
| 恶意插件包或能力扩大 | 安装检查、包摘要、能力和安装脚本展示、可信代码确认、归档资源上限 |
| Chromium、SQLite、OneBot11 或插件运行时故障 | readiness、diagnostics、结构化错误、恢复摘要和受控重试 |

## 发布验收

正式发行物必须通过：

- strict contracts、valid/invalid fixtures、embedded schema 和 generated types drift；
- server tests、目标包 `-race`、server build 与 binary-mode govulncheck；
- Web 与 Launcher typecheck、test、build 和受影响的 E2E；
- Node.js SDK dist drift、Python SDK 与运行时客户端 wheel fresh-venv import；
- 四种归档的 `LICENSE`、`THIRD_PARTY_NOTICES.md`、metadata、artifact smoke 和 recovery drill；
- doctor、agent docs、文档链接和 `git diff --check`。

## 更新安全验收

更新核心必须拒绝：错误 key、错误签名、过期或重放 manifest、降级、同版本不同摘要、artifact hash/size/platform/version/signer 不匹配、路径穿越、reparse point、大小写冲突、额外根目录、zip bomb、磁盘不足和超时。

事务恢复测试必须覆盖每个 journal phase、helper 启动失败、停止服务失败、健康检查失败、回滚成功和回滚失败。升级前后的 `config/user.yaml`、`data/**` 和 `plugins/installed/**` 必须保持一致。

Windows packaged E2E 必须使用正式签名证书覆盖：

- 服务运行时的真实 N→N+1；
- 服务停止时的真实 N→N+1；
- 强制 postflight 失败后的旧版与旧状态恢复；
- Launcher、server、updater 的 `signtool verify /pa /all`。

正式 Authenticode 证书与真实签名 packaged E2E 是 Phase 3 的阻塞门槛。证书可用前，`windows-x64-full` 保持 `guided`，不能用自签名证书替代验收。

## 产品验收

- OneBot11 `reverse_ws`、`forward_ws`、`http_api` 和 `webhook` 具备可观察的连接、鉴权与失败状态。
- 首次初始化、cookie 会话、CSRF、WebSocket Origin 和 Launcher control token 的安全用例全部通过。
- 插件安装、卸载、启停、重载、自定义管理页动作、日志和任务终态可追溯。
- 任务队列满时不产生 orphan task；重启把未完成任务收敛为 interrupted。
- 插件事件、调度触发、出站消息、渲染和恢复共享正式状态与错误语义。
- Web、Launcher 和 CLI 不维护服务端状态的平行副本。
- WCAG 2.2 AA 的键盘、焦点、ARIA、对比度、reduced-motion 和目标视口验收通过。

## 正式边界

多协议、多实例、高可用、插件市场和插件 OS 强沙盒不属于本次发布门槛。第三方插件是用户明确确认后执行的完全可信本地代码。
