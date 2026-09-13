# Implementation Order

本文档定义长期依赖顺序和跨层边界。已交付版本的完成状态见 [`../CHANGELOGS/`](../CHANGELOGS/README.md)。

## 1. 固定正式契约

正式语义变化与实现修复的先后规则见根 [`AGENTS.md`](../../AGENTS.md) 的 Hard Rules。

契约应同时固定：

- 字段、状态、错误码和兼容语义；
- 鉴权、权限与信任边界；
- 硬上限、超时和失败终态；
- `x-fixtures` 或等价样例引用。

## 2. 建立验证样例与生成链

每个正式 surface 至少提供能证明关键行为的 valid、invalid 和边界样例。strict validator 必须按声明的 JSON Schema/OpenAPI 版本验证，并拒绝网络 `$ref`。

契约变更按实际影响检查并更新：

- fixtures 和 examples；
- embedded schemas；
- OpenAPI/WebSocket generated types；
- SDK 输入输出模型；
- drift gate。

生成链以各自输入依赖为准；OpenAPI 的客户端类型与 Go service/model 的 Wails bindings 分别生成，未受影响的生成链不需要重新生成。

## 3. 固定状态归属与持久化语义

在接入业务路径前明确状态的归属、生命周期和并发语义：

| 状态 | 职责方 | 正式来源 |
| --- | --- | --- |
| 配置 | Config service | default 与 user 配置的校验后快照 |
| 持久业务状态 | Server domain service | SQLite 与当前初始化结构 |
| 共享运行状态 | 对应 Server service | 锁或原子快照保护的内存状态 |
| 插件声明 | Plugin Catalog | 校验后的 manifest、管理页入口、安装来源与 package metadata |
| 插件商店 | Plugin Store Service | 来源配置、最后成功目录缓存与安装来源身份 |
| 插件进程状态 | Runtime Manager | runtime snapshot |
| 后台任务 | Task Registry | 有序持久化记录与终态 |

数据库结构变更先更新当前 schema，再按实际影响同步 queries、生成物、fixtures 和恢复说明。普通状态修复不能引入平行数据库或客户端状态来源。

## 4. 实现服务端领域语义

Server 负责正式业务状态、并发控制、资源边界、错误映射和持久化。实现应满足：

- 密码哈希、网络 I/O 和数据库 I/O 不占用全局状态锁；
- 共享可变状态具有明确锁或原子快照；
- 队列 admission 发生在创建持久任务之前；
- 任务、调度和安装具有单一串行 mutation path；
- 下载、HTTP、归档、展开和插件包具有硬上限；
- 失败返回正式错误码，不泄露凭据或 token 状态。

## 5. 接入协议、插件和平台能力

各组件的职责与禁止事项见 [Platform Architecture](../architecture/platform-architecture.md)，插件进程的信任与能力边界见 [Plugin Runtime](../architecture/plugin-runtime.md)；接入时不得跨越这些边界。

协议扩展先更新对应正式 schema，再按实际影响同步 fixtures、SDK 和示例插件。

## 6. 暴露管理与本机控制入口

领域语义确定后，实现管理 HTTP/WebSocket 接口。Handler 只负责 transport、鉴权、参数校验和领域错误映射。

- 浏览器会话使用 Host-only HttpOnly cookie、CSRF 和 Origin 校验。
- Bearer token 服务非浏览器客户端。
- setup token 和 Launcher control token 属于独立一次性或进程级凭据。
- WebSocket 不接受 query token。
- Launcher 本机控制不等于浏览器管理员会话。

## 7. 接入 Web、Launcher、CLI 与 SDK

- Web 只消费正式 API/WebSocket，不保存 bearer token。
- Launcher 负责本机进程、系统集成、更新确认与安装编排，不复制 Web 业务页或 server 状态机。
- CLI 复用 server/update 核心，提供离线、脚本化和恢复入口。
- SDK 只暴露正式协议；生成物由 CI 检查修改、删除和新增漂移。

客户端接入时使用契约定义的状态名、错误码和字段。

## 8. 打包与恢复

发布实现依赖稳定的 contract、server、客户端和 SDK：

- 归档包含正式运行资源、LICENSE 和经审阅的第三方 notices；
- release metadata 按正式 schema 生成；
- 正式 smoke 同时验证新装和备份恢复。

## 9. 验收与发布

发布前按受影响面完成以下验收；日常改动的最小验证见[质量门禁](./quality-gates.md#按改动面的最小验证)：

- strict contracts 与 generated drift；
- 目标包 `-race`、server tests/build 和 binary vulnerability scan；
- Web/Launcher typecheck、test、build 与风险对应的 E2E；
- SDK 打包与 fresh-environment install；
- release artifact、license notice、smoke 与 recovery drill；
- doctor、文档链接和 `git diff --check`。

只有 exit code 不能证明真实产物时，必须继续检查生成文件、归档内容或运行时效果。

## 独立设计边界

以下方向需要新的 contract、状态一致性说明和验证矩阵，不能作为日常修补隐式进入主流程：

- 多实例与高可用；
- 插件 OS 强沙盒；
- 当前 OneBot11 与 QQ 官方之外的聊天协议；
- 新的官方插件运行时；
- 新的客户端状态来源或远程组件运行时。
