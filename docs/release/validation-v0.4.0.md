# 0.4.0 候选版验收与交付记录

本轮按全新分发前提完成执行计划 P01–P26，交付四种已验收候选包及校验材料；尚未创建发布 tag 或公开 Release。验证范围、平台签名和工具限制见下文，候选验收完成不代表这些边界之外的验证也已完成。

## 来源与验证入口

| 对象 | 已核实来源与结果 |
| --- | --- |
| 生产实现与四个归档 | `5b2fa9d85212a2129784d07088f4d844cefe09ed`，版本 `0.4.0` |
| 完整 CI | [run 34539781821](https://github.com/RayleaBot/RayleaBot/actions/runs/34539781821) 全部通过：Linux Server 全量测试、核心 race、lint，Windows 安装/锁/真实浏览器，Web、Launcher、SDK、契约、生成与双平台脚本门禁 |
| 原生构建与实际包流程 | [run 34539781886](https://github.com/RayleaBot/RayleaBot/actions/runs/34539781886)，Windows x64、Linux x64 桌面/服务端、macOS ARM64 四项全部通过 |
| 发布工具修复 | `45508122283baae14d54b255ad27df561bcb3171`，从各归档的实际字节计算资源清单摘要；108 项发布脚本测试、5 项工作流测试及四个真实归档正反验证通过 |
| 修正元数据与重新签名 | [run 34543000042](https://github.com/RayleaBot/RayleaBot/actions/runs/34543000042)，工具工作流 `d901466d1b3144ba30eb8d03fc0e8e09d6af7750`；先验证原生构建 run 的仓库、提交与成功状态，再下载原四包生成、签名及验证元数据 |
| 后续样例与文档 | `d5493776` 只修正调度成功样例和两份生成测试向量，生成 verify、strict、Server/独立 SDK 向量测试通过；文档及发布工具更新均未改变上述四个归档的生产编译输入 |

本机交付目录为 `dist/candidates/v0.4.0/5b2fa9d8/`，入口为其中的 `README.md`。`candidate-status.json` 汇总状态，`independent-artifact-verification.json` 记录独立检查，`validation-evidence-*` 保存原生执行日志。

## 四种最终候选包

四个归档均完成 archive smoke、首次配置/管理员初始化、当前版本备份恢复及两次恢复后启动观察，每次观察 300 秒；self-host 持续运行窗口为 600 秒、探测间隔 30 秒。下表耗时包含相应流程准备与收尾，渲染上限仍为默认 30 秒。

| 产物 | SHA-256 | 字节数 | 恢复 / self-host 实际耗时（秒） |
| --- | --- | ---: | ---: |
| `windows-x64-full` | `1eb130d00c2a90497b3e5c96d11348e13b144cb079904bf594ba5ebd714e67f3` | 45,494,942 | 616.418 / 681.250 |
| `linux-x64-full` | `3d720ff3559f0af9a77700eda96b55d00566b417f4e918b4c079cb6b402caf45` | 39,362,312 | 602.076 / 681.219 |
| `macos-arm64-full` | `c83e0333df532c24832348494430c7400486145449980d82cc578fb6da0542bb` | 38,781,513 | 602.673 / 644.685 |
| `linux-x64-server` | `af55ab431b2249ab2a8176830371f7f4400272f75aca0d84902ebc325ef93c60` | 29,598,452 | 601.924 / 679.642 |

每个平台均从实际包运行 Server 和该平台原生 SDK 插件，完成安装、启用、HTTP 设置、重载、禁用、卸载；重载前后各生成一张有效的 960×411 PNG，插件进程按实际 PID 核对退出。分钟 cron 由调度器真实触发，回调来源为 `scheduler/scheduler.internal`，任务 ID、插件 PID、`last_run` 和成功计数相互对应；卸载后任务消失。Server 退出后所属浏览器进程及 profile 均回收。

流程入口为 [self_host_smoke.py](../../scripts/release/self_host_smoke.py) 和 [recovery_drill.py](../../scripts/release/recovery_drill.py)。安装、卸载后置失败与回滚另由[真实文件/进程故障回归](../../server/internal/plugins/lifecycle/install_failure_test.go)覆盖，不冒充正式签名 Windows 自动更新回滚。

## 清单、摘要与签名

原汇总清单错误地将 Ubuntu 源文件的 LF 字节摘要写入 Windows 产物，而该包中的 `.deps/manifest.json` 为 CRLF。JSON 内容相同并不能代替原始字节摘要相同。修复后按每个实际归档计算并检查内层摘要；即使清单签名有效，错误的内层摘要也会使发布工具验证失败。

修正过程中四个归档字节均未改变。受控 CI 使用原 `5b2fa9d8` 源码构建验证器验签；本机另用交付 Windows 包中的 Server 对四个归档逐一验签，并独立检查归档 SHA/大小、内层清单 SHA、实际文件数/展开大小、包内版本/提交/说明链接以及完整执行证据，全部通过。

| 当前校验文件 | SHA-256 |
| --- | --- |
| `release_manifest.v2.json` | `d6762be4c3317e74040a13a6905f6641d15e6a39f3275892dee1b19d041e1415` |
| `release_manifest.v2.sig.json` | `14aa39defa64584eb76090f72544e1f1262a7c5fbdf88df25ace6620321cff46` |
| `SHA256SUMS.txt` | `8f5ea75bf2acc9bfdc9cf8acca62ffc4fdaa2821a2934cabbe1fbbf809f473ae` |

签名算法为 Ed25519，公开 `key_id` 为 `release-2026-primary`。当前文件位于交付目录的 `release-metadata/`。含错误内层摘要的原三文件只保留在 `.verification/original-release-metadata/` 供审计，不是交付校验入口。清单签名与平台可执行文件签名分别记录。

## Windows 原生界面

最终 ZIP 的 SHA 与上表一致。使用包内 Wails 程序、实际 WebView2 和独立配置/数据库/profile，已完成首次初始化、原生 Start/Stop、改测试端口后重启、关闭确认 Cancel、隐藏后 health 保持、Explorer 真实鼠标点击托盘恢复，以及原生完全退出。Launcher、Server、6 个 WebView2 和 conhost 共 9 个所属进程均退出，端口释放；未用强杀完成验收，也未操作用户浏览器。

报告和截图位于 `native-ui-evidence/report.md`、`native-ui-evidence/13-restored-running-window.png`；证据包 `native-ui-evidence.zip` 的 SHA-256 为 `ea1fd8a33317b7d06e5ff7948ab093515e190ad50f9a485a2c9acf9a958c7b0f`。退出时 WebView2 记录过 `Chrome_WidgetWin_0` 注销错误 1412，随后已核实全部进程退出，没有残留。

**第二实例的本轮重复复验未执行。** 启动命令被工具自动审批拒绝，仅返回 `CreateProcess rejected: blocked by policy`，没有更具体原因；没有绕过或重试。此前 `db938746` 实际包已验证第二实例退出 0、唤起原窗口并保留原 Server。到最终包之间 Launcher Go、UI、资源、生成绑定与锁文件未变，变化限于 Wails CLI 构建脚本及参数测试；但两个 Launcher 二进制摘要不同，历史证据不能标成本轮执行或同一二进制验证。该范围作为 P23 完成记录的明确限制保留。

## 其他行为与回归证据

| 范围 | 实际验证 |
| --- | --- |
| 多适配器 | [实例路由测试](../../server/internal/app/adapter_routing_test.go)与[HTTP 集成](../../server/tests/integration/adapters_http_test.go)通过真实 App 和本地协议对端覆盖两个实例、父事件约束、身份/缓存隔离、按实例鉴权；未连接外部 OneBot 或 QQ 官方公网账号 |
| 设置与运行时 | [真实 App 组合测试](../../server/tests/integration/service_composition_test.go)贯通 HTTP 设置、原生插件、配置通知与分发；末帧/进程退出旧实现确定失败，修后 100 轮末帧、关键 race 和完整 CI 通过 |
| Web | 455 单测、6 项生产浏览器流程、14 项交互用例通过；最终实现另跑 5 项现有窄屏、减少动画、高对比度和键盘用例通过，日志 `p22-accessibility-closeout.log` |
| Launcher | Go 正式 API 校验、绑定生成、状态文件损坏、关闭错误回归与 62 JS 测试、4 Renderer E2E 通过；Windows 原生 UI 与 Linux/macOS 原生构建分开记录 |
| SDK/示例 | SDK race、协议向量、生成检查与 10 个示例独立构建通过；部分示例为无测试函数的 main 包，不宣称每个示例完整业务运行已测 |
| 资产/字体 | 来源授权、离线资源与 1023 个中文样本的真实字体检查已记录；不是全部 Unicode 字形或全部平台的完整 WCAG 认证 |

## 验收发现并修复的问题

| 提交 | 修复与证明 |
| --- | --- |
| `60e65f77`、`07a365d1` | 原生插件验收使用正式 `source_digest` 与实际同步生命周期响应，真实验证 PNG、进程与目录回收 |
| `db938746` | FFmpeg 清单改用仍保留的同版本线月度归档；真实下载、官方摘要和平台入口核验通过 |
| `9f49b885`、`80aaa1c7` | Edge 兼容重启脱离进程所有权、启动取消回收、Windows 文件 URL 丢盘符等问题均有真实回归 |
| `8e43b34f`、`44ff2dc1` | Linux Wails CLI 使用正确 GTK 标签；macOS 系统 Bash 保留无可选参数时的完整打包调用 |
| `30734d80` | runtime 显式拥有输出管道，退出与 Stop 有界等待最后协议帧，保留合法结果及正式协议错误 |
| `b4a23bde`、`c1f1993f` | Windows 解包目录锁重试；实际 cron 到原生插件、成功统计及卸载清理进入四平台门槛 |
| `65b92247`、`898a28b7` | 启动遵守调用方预算；图片解码/字体 Promise 真正等待；每个页面独立保持资源处理可运行，真实延迟资源与并发回归通过 |
| `5b2fa9d8` | 只在实际页面激活与截图期间协调同一浏览器，等待可取消、失败释放位置，资源加载保持并发；最终 Windows CI 与实际包完整流程通过 |
| `45508122` | 发布元数据取归档内资源清单的字节摘要，并验证内部一致性；签名有效但字段错误的负例与四个真实包检查通过 |

## 验证边界与分发方式

- Windows 三个 PE 均为 NotSigned，保持 `guided`；正式 Authenticode 与签名 packaged 自动更新 E2E 不在本轮通过范围。详见[更新安全门槛](./acceptance-and-risks.md#更新安全验收)。
- 最终 macOS Server 和 Launcher 为链接器 ad-hoc 签名，CodeDirectory flags 为 `0x20002`，逐页摘要核验通过；没有 Developer ID/CMS/Team ID 或 bundle 资源签名封装，公证与 Gatekeeper 未验收。
- Linux/macOS 已验证原生构建与实际服务/插件/渲染/恢复流程，没有同等的原生桌面人工操作记录；外部聊天平台实网验证也没有冒充为通过。
- 最终 Windows/macOS Server 均使用 `govulncheck@v1.7.0`、Go 1.26.6 执行 binary-mode 扫描，受影响符号和导入包漏洞均为 0；仍有 3 项仅存在于声明模块的 x/crypto SSH/OpenPGP 公告（GO-2026-6355、GO-2026-6354、GO-2026-5932），不等于全局没有漏洞。macOS 二进制仅解析，未在 Windows 上执行。证据为 `.verification/vulnerability/results.json`。
- 当前交付是未公开分发的候选版。未生成发布 tag、公开 Release 或提前编造正式 CHANGELOG；安装与本版恢复步骤见[候选说明](./notes/v0.4.0.md)。
