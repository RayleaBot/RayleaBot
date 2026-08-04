# Implementation Order

本文档定义长期依赖顺序和跨层边界。版本范围与完成状态见 [`../execution-plan-v0.3.md`](../execution-plan-v0.3.md)。

## 1. 冻结正式契约

任何新增或变更的 HTTP、WebSocket、schema、错误码、事件、插件协议、CLI 或 release metadata 必须先进入 `contracts/`。

契约应同时固定：

- 字段、状态、错误码和兼容语义；
- 鉴权、权限与信任边界；
- 硬上限、超时和失败终态；
- `x-fixtures` 或等价样例引用。

实现、README、fixtures 和 examples 不能反向裁决 contract。

## 2. 建立验证样例与生成链

每个正式 surface 至少提供能证明关键行为的 valid、invalid 和边界样例。strict validator 必须按声明的 JSON Schema/OpenAPI 版本验证，并拒绝网络 `$ref`。

契约变更同轮更新：

- fixtures 和 examples；
- embedded schemas；
- OpenAPI/WebSocket generated types；
- SDK 输入输出模型；
- drift gate。

## 3. 固定状态 owner 与持久化语义

在接入业务路径前明确状态的 owner、生命周期和并发语义：

| 状态 | Owner | 正式来源 |
| --- | --- | --- |
| 配置 | Config service | default 与 user 配置的校验后快照 |
| 持久业务状态 | Server domain service | SQLite 与正式 migration |
| 共享运行状态 | 对应 Server service | 锁或原子快照保护的内存状态 |
| 插件声明 | Plugin Catalog | 校验后的 manifest、管理页入口、安装来源与 package metadata |
| 插件商店 | Plugin Store Service | 已验证静态目录、目录摘要与发布者身份 |
| 插件进程状态 | Runtime Manager | runtime snapshot |
| 后台任务 | Task Registry | 有序持久化记录与终态 |
| 更新事务 | Updater | 签名 metadata、最高版本记录与 journal |

数据库结构变更必须先更新 schema/migration、queries、fixtures 和恢复说明。普通状态修复不能引入平行数据库或客户端状态源。

## 4. 实现服务端领域语义

Server 负责正式业务状态、并发控制、资源边界、错误映射和持久化。实现应满足：

- 密码哈希、网络 I/O 和数据库 I/O 不占用全局状态锁；
- 共享可变状态具有明确锁或原子快照；
- 队列 admission 发生在创建持久任务之前；
- 任务、调度和安装具有单一串行 mutation path；
- 下载、HTTP、归档、展开和插件包具有硬上限；
- 失败返回正式错误码，不泄露凭据或 token 状态。

## 5. 接入协议、插件和平台能力

- Adapter 只负责平台协议、连接状态、事件归一化和动作投影，不直接写业务状态。
- Event Ingress 负责命令解析和聊天治理；Bridge 负责统一事件结构校验。
- Dispatcher 是插件事件排队和出站 action 的唯一执行出口。
- Runtime Manager 只负责插件进程、JSONL 协议和生命周期。
- Plugin Store Service 只消费签名目录并复用统一 Installer，不直接写运行目录或信任 manifest 自报身份。
- Local Action Service 是插件访问配置、存储、调度、渲染、HTTP、治理和 OneBot 扩展的唯一入口。
- Scheduler 只触发插件事件，不直接发送消息。
- Render Service 是平台统一渲染入口，插件不维护独立浏览器链路。

协议扩展必须同步正式 schema、fixtures、SDK 和示例插件。

## 6. 暴露管理与本机控制入口

管理 HTTP/WebSocket 在领域语义稳定后接线。Handler 只负责 transport、鉴权、参数校验和领域错误映射。

- 浏览器会话使用 Host-only HttpOnly cookie、CSRF 和 Origin 校验。
- Bearer token 服务非浏览器客户端。
- setup token 和 Launcher control token 属于独立一次性或进程级凭据。
- WebSocket 不接受 query token。
- Launcher 本机控制不等于浏览器管理员会话。

## 7. 接入 Web、Launcher、CLI 与 SDK

- Web 只消费正式 API/WebSocket，不保存 bearer token，不从日志推断状态。
- Launcher 负责本机进程、系统集成、更新确认与安装编排，不复制 Web 业务页或 server 状态机。
- CLI 复用 server/update 核心，提供离线、脚本化和恢复入口。
- SDK 只暴露正式协议；生成物由 CI 检查修改、删除和新增漂移。

客户端接线不能引入新的状态名、错误码、字段别名或信任根。

## 8. 打包、签名与恢复

发布实现依赖稳定的 contract、server、客户端和 SDK：

- 归档包含正式运行资源、LICENSE 和经审阅的第三方 notices；
- release metadata 由正式 schema 生成并签名；
- Windows 自动安装同时要求 Ed25519 和 Authenticode；
- update helper 位于安装根之外，使用 journal、offline backup、同卷 staging 和原子 swap；
- 正式 smoke 同时验证新装、升级、恢复和故障回滚。

缺少正式证书、签名或 recovery drill 时，发布方式必须降级为 guided/manual，不能放宽门禁。

## 9. 验收与发布

按受影响面运行最小但充分的验证：

- strict contracts 与 generated drift；
- 目标包 `-race`、server tests/build 和 binary vulnerability scan；
- Web/Launcher typecheck、test、build 与风险对应的 E2E；
- SDK 打包与 fresh-environment install；
- release artifact、签名、license notice、smoke 与 recovery drill；
- doctor、文档链接和 `git diff --check`。

只有 exit code 不能证明真实产物时，必须继续检查生成文件、归档内容或运行时效果。

## 独立设计边界

以下方向需要新的 contract、状态一致性说明和验证矩阵，不能作为日常修补隐式进入主链：

- 多实例与高可用；
- 插件 OS 强沙盒；
- 非 OneBot 多协议；
- 新的官方插件运行时；
- 新的客户端状态源、远程组件运行时或发布信任根。
