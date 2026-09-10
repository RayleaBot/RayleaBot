# 0.4.0 候选版验收记录

本页记录全新分发候选版的实际验证范围与剩余门槛，供交付核对使用。版本为 `0.4.0`，尚未公开分发；Windows 保持 `guided`。P01、P23、P24、P26 仍待最终结果登记，不能据此认定全部优化或正式发布验收完成。

验收标准见[验收与风险](./acceptance-and-risks.md)，工作包状态见[执行计划](../execution-plan-v2.md)。最终交付必须将源码提交、归档摘要、原生平台与验证结果对应起来；以下早期候选包的通过记录不自动转移到后续构建。

## 已验证的产品链路

| 范围 | 已核实结果与可重复入口 | 验证边界 |
| --- | --- | --- |
| 多适配器 | [实例路由测试](../../server/internal/app/adapter_routing_test.go)使用两个本机 HTTP 对端，核对相同目标 ID 的选路、父事件约束、身份缓存隔离及另一实例零请求；[管理集成](../../server/tests/integration/adapters_http_test.go)覆盖集合状态和按实例鉴权的入站请求 | 已验证真实 App 与本地网络对端；未登记连接外部 OneBot 实现或 QQ 官方公网账号的验收 |
| 插件设置与运行时 | [真实 App 组合测试](../../server/tests/integration/service_composition_test.go)贯通 HTTP 设置、原生 SDK 插件、配置通知与入站分发；runtime、lifecycle、dispatch、App、HTTP 集成和 WS 的 Windows race 通过 | 平台与源码范围以各次记录为准，不能代替最终包验证 |
| 插件完整管理流程 | `9f49b885` Windows 实际包使用修正后的验收脚本，从空目录完成安装、启用、HTTP 设置、重载、禁用和卸载；两个原生插件进程先后退出，安装目录最终消失 | 本机功能观察窗口为 1 秒、探测间隔 1 秒，使用已下载并校验摘要的运行资源缓存；不计为正式长时或本轮重新下载验收 |
| 图片与浏览器回收 | 同一次包验收中，原生插件通过正式 SDK 两次渲染内置 `help.menu`，产生 PNG 签名有效的 960×411 图片及可关联日志；Server 关闭后所属浏览器进程和 profile 均释放。[真实 Edge 回归](../../server/internal/render/edge_windows_test.go)另校验 PNG、CDP 实际 PID、启动取消和重复关闭 | Edge 本机回归已通过；Windows CI 的系统 Chrome 首次启动或渲染超时仍需解决，不能据本机成功注销该缺口 |
| 安装、卸载与失败回滚 | [安装失败回归](../../server/internal/plugins/lifecycle/install_failure_test.go)、[安装事务](../../server/internal/plugins/lifecycle/install_service_test.go)及 Windows 文件占用回归覆盖真实临时文件、状态恢复、后置失败和恢复文件保留；相关 race 通过 | 故障由测试注入；不代表正式签名 Windows 自动更新的 N→N+1 回滚验收 |
| 配置、登录与 Web | [生产浏览器配置测试](../../web/tests/production/config.real.spec.ts)等覆盖真实 Server 配置保存、认证与运行反馈；已记录 455 项 Web 单测、6 项生产浏览器用例和 14 项交互用例 | 浏览器测试、键盘/焦点与响应式回归不等同于全部平台的完整 WCAG 人工验收 |
| 日志与调度 | [日志 HTTP 集成](../../server/tests/integration/logs_http_test.go)与历史分页测试覆盖查询和归属；[调度领域测试](../../server/internal/scheduler/scheduler_test.go)覆盖真实 SQLite、触发回调和状态，另有时区/视图回归；包验收核对插件日志与安装、卸载任务终态 | 完整产物中的原生插件定时任务触发到出站完成链路尚未单独登记 |
| 本版初始化与恢复 | 本机空目录、默认及自定义数据库路径恢复通过；托管 Windows `80aaa1c7` 包的 recovery drill 通过，记录首次 setup、配置及插件数据保留、恢复后登录与重复启动幂等 | 使用当前 backup manifest v3 与数据库 `000001`；该包随后 self-host smoke 失败，不能记为整个产物通过 |

完整包流程入口为 [self_host_smoke.py](../../scripts/release/self_host_smoke.py)，初始化恢复入口为 [recovery_drill.py](../../scripts/release/recovery_drill.py)。正式托管门槛保持 recovery 观察窗口 300 秒、self-host 观察窗口 600 秒、探测间隔 30 秒。

## Windows 原生桌面补验

本机原生 UI 使用的实际 ZIP 源码为 `db938746a9cf69a9b38c3a7eca095646ed568a8a`，归档 SHA-256：

```text
2bb54a85e19b28560a005a4c8628b269f52b585e282e9016ba5e313ca4b4698c
```

使用包内 Wails 程序、实际 WebView2 和独立运行目录，已验证：

- 缺少配置时首次 Start 自动初始化，Stop 后修改测试端口并再次启动。
- 原生关闭确认中的 Cancel 保留窗口和服务；隐藏窗口时服务 health 保持正常。
- 停服和运行状态下，实际点击 Explorer 托盘图标均恢复原窗口；第二实例退出码为 0，并唤起原窗口。
- 打开真实浏览器管理页后，setup token 不留在地址中；本轮没有创建管理员。
- 在确认框选择 Exit 后，Launcher、Server、6 个 WebView2 进程和 conhost 全部退出，监听端口释放；未使用强制终止完成验收。

证据归档名为 `native-ui-evidence.zip`，包含 `report.md`、`10-native-package-window.png`、UIA、进程树和签名状态记录；归档 SHA-256 为 `1d96f40cb7b271ff951da7d1748a1387462314ce2e41221d6be4d827f6051cbd`。最终交付位置待登记。

该补验属于上述具体候选包。三个 PE 文件均为 NotSigned，未验证正式签名安装或自动事务更新。退出时记录过 WebView2 `Chrome_WidgetWin_0` 注销错误 1412，但所属进程最终全部退出；最终包仍需核对这一诊断。

## 验收发现的修复

| 提交 | 修复及验证 |
| --- | --- |
| `60e65f77`、`07a365d1` | self-host 使用正式 `source_digest`，拒绝旧 `revision_id`；插件启用、重载、禁用按 `200 + PluginDetail` 验证，安装/卸载仍按异步任务处理；正例读取正式 fixtures |
| `9f49b885` | Windows Edge 显式跳过兼容层重新启动，使 allocator 持有实际浏览器 PID；同时回收启动取消的进程。六组独占 profile 实验证明默认参数会换 PID 并提前关闭输出管道；真实 Edge 出图与取消 race 回归通过 |
| `80aaa1c7` | Windows 文件 URL 保留盘符，修复跨盘运行时资源读取失败；浏览器文档与图片 artifact 使用相同文件 URL 规则 |
| `8e43b34f`、`44ff2dc1` | Wails CLI 本身按 Linux GTK3 标签构建；macOS Bash 保留完整打包参数。Linux Launcher 的生成、类型检查、测试和构建已在第二轮 CI 通过 |
| `30734d80` | 插件 runtime 显式持有输出管道，退出和 Stop 在有界期限内等待最后协议帧，避免合法结果丢失或非法动作误报通用错误。旧实现确定性回归失败；修后 100 轮末帧回归、10 轮关键 race、相关六组包 race 和 runtime lint 通过 |

上述修复后的最终源码与全部产物必须重新对应登记，不能沿用早期归档摘要作为最终交付证明。

## 托管检查与最终登记

| 执行 | 实际结果 |
| --- | --- |
| [首轮 CI](https://github.com/RayleaBot/RayleaBot/actions/runs/34532245745)，`9f49b885` | Linux Server 全部门禁、Web、SDK、契约、双平台脚本检查通过；Launcher GTK 标签和 Windows render 测试失败 |
| [第二轮 CI](https://github.com/RayleaBot/RayleaBot/actions/runs/34533310868)，`80aaa1c7` | Linux Launcher 全部通过，Windows 丢盘符用例不再失败；Linux runtime 暴露末帧/退出竞态，Windows 通用 Chromium 测试仍启动超时，整轮失败 |
| [第二轮产物验收](https://github.com/RayleaBot/RayleaBot/actions/runs/34533311035)，`80aaa1c7` | 已核对 Windows package、archive smoke、recovery drill 通过；self-host 在首次渲染阶段因 Chromium 启动超时失败，未完成 600 秒观察。其余平台及后续构建结果由最终登记替换 |
| 最终 CI | 待维护者登记最终源码 SHA、run 链接及 required job 全通过结果 |
| 最终产物验收 | 待维护者登记同一源码的四种归档、SHA-256、验证证据与实际观察窗口 |

| 最终目标产物 | 源码提交 / SHA-256 | 原生构建与 archive smoke | 300 秒恢复 / 600 秒 self-host |
| --- | --- | --- | --- |
| `windows-x64-full` | 待登记 | 待最终结果 | 待最终结果 |
| `linux-x64-full` | 待登记 | 待最终结果 | 待最终结果 |
| `macos-arm64-full` | 待登记 | 待最终结果 | 待最终结果 |
| `linux-x64-server` | 待登记 | 待最终结果 | 待最终结果 |

Windows/macOS 本机 binary-mode govulncheck 已记录 0 个受影响符号、0 个导入包漏洞；另有 3 项仅位于声明模块且未被调用的公告。最终二进制扫描结果仍随最终产物登记。

## 剩余门槛与责任

| 门槛 | 当前边界 | 责任与下一步 |
| --- | --- | --- |
| 最终 CI 与四种候选产物 | 早期 CI/产物存在明确失败；修复不能替代通过结果 | 发布维护者核对最终 run、源码 SHA、摘要、证据文件及完整观察窗口，再回写 P01/P23/P24/P26 |
| 原生桌面平台范围 | 已有 Windows 实包 UI 补验；Linux/macOS 原生交互没有同等记录 | 发布维护者登记原生构建与实际界面范围，保持未验证交互可见 |
| 外部聊天网络与完整调度链路 | 本地协议对端与领域回归已覆盖；外部 OneBot、QQ 官方公网及产物级调度出站未登记 | 对应接入维护者使用实际配置补验，记录平台、实例和可观察结果，不将模拟对端写成实网 |
| 正式 Windows 签名更新 | 无正式 Authenticode 证书及签名 packaged E2E | 保持 `guided`；证书可用后按[更新安全验收](./acceptance-and-risks.md#更新安全验收)执行，Ed25519 清单签名不替代此门槛 |
