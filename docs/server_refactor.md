# Server Structure Baseline

本文档记录 `server/` 的结构边界、依赖方向和验证入口。

## 结构边界

| 路径 | 职责 |
| --- | --- |
| `server/internal/app` | 应用生命周期、关闭流程、运行状态和 composition root |
| `server/internal/app/platform` | 配置加载后的基础设施组装，包括 SQLite、auth、secret、tasks、scheduler、logging、console |
| `server/internal/app/pluginstack` | 插件状态组装，包括 catalog、plugin repository、settings、KV、file store、install、uninstall、webhook registry 和 plugin log limiter |
| `server/internal/app/renderstack` | Render Service、浏览器路径准备、插件模板同步和渲染资源关闭 |
| `server/internal/app/eventstack` | OneBot11 shell、Bridge、Dispatcher、ReplyTarget、OutboundLimiter 和 dispatcher observability flush |
| `server/internal/app/actionwire` | local action 对 scheduler、config changed、secret reader 和 renderer 的内部适配 |
| `server/internal/app/servicegraph` | 应用服务组装，包括 local actions、runtime registry、system、plugin lifecycle、event ingress、protocol、webhook、governance、third-party |
| `server/internal/app/httpwire` | HTTP server、management handlers、WebSocket handlers 和路由模块组装，直接消费插件、事件与渲染输入 |
| `server/internal/bootstrap` | `cmd/raylea-server` 与 `internal/app` 之间的入口装配层 |
| `server/internal/management/router` | 管理面公共路由、受保护路由、管理 UI fallback 和健康检查 |
| `server/internal/management/*api` | 各管理面 HTTP API 子域的 handler、request、response 和路由注册 |
| `server/internal/management/pluginapi/view` | 插件管理 API 的列表、详情和 dead-letter response 投影 |
| `server/internal/management/ws` | 管理面 WebSocket 入口 |
| `server/internal/management/events` | 管理面 WebSocket event frame 与 payload 投影 |
| `server/internal/plugins/actions` | 插件 local action registry 和公共入口 |
| `server/internal/plugins/actions/*action` | 单类 local action 的执行逻辑和依赖接口 |
| `server/internal/integrations/bilibili/session/identity` | Bilibili 请求身份、UA、语言和请求头生成 |
| `server/internal/integrations/bilibili/{session,captcha,fingerprint,proxy}` | Bilibili 登录、扫码和账号资料读取的独立协作者 |
| `server/internal/logging/repository` | 管理日志 SQLite 持久化和查询实现 |
| `server/internal/system/startup` | 启动运行环境阶段、标签、日志字段和失败归因 |
| `server/internal/bot/adapter/onebot11/shell/backoff` | OneBot11 shell 重连退避算法 |

## 包职责原则

职责边界由**依赖方向**和**类型内聚**表达，不由文件数或行数表达。

- 一个类型的方法散落到多个文件，不构成职责边界。包内职责过重时，按职责抽出**协作者类型**或**子包**（各自持有自身状态与测试），让原类型退化为协调器；不要按方法前缀把同一类型切成更多文件。
- 同一职责被拆成多个微文件（例如把 handler 与其专用 request/response 类型拆成 `_handlers.go` + `_types.go`）时，合并回同一文件。
- 生成物、测试工具和构建产物不承担业务边界职责。

`server/tests/architecture` 维护结构回归检查。其中行数检查仅作为拦截病态文件的宽松安全网（单个生产 Go 文件 ≤ 1500 行），不作为拆分驱动指标。

## 依赖方向

- `contracts/` 是 HTTP、WebSocket、schema、错误码和发布元数据的正式来源。
- `management/*api` 只承担管理面入口和 DTO 投影，不作为领域服务模型来源。
- composition root 单向装配：`platform → pluginstack → renderstack → eventstack → servicegraph → httpwire`。下层不得 import 上层，只有 `internal/app` 可同时 import 这些子包；`actionwire` 是 service assembly 使用的内部适配 helper。
- 领域包不得 import composition root：除入口/装配层（`internal/app`、`internal/bootstrap`）、测试 harness（`internal/testapp`）和 `server/tests/**` 外，`internal/` 下生产代码不得 import `internal/app` 或 `internal/app/httpwire`。
- `server/internal/app/httpwire` 负责把运行时配置投影为管理面 handler 所需配置。
- `auth` 持有认证基础设施；管理面登录 handler 只消费认证接口和登录失败计数接口。
- `plugins/actions` 通过 registry 分发 local action；每个 action 子包只持有自身需要的依赖。
- `server/internal/app/pluginstack` 只承接插件状态和插件仓库类对象，不 import adapter、bridge、dispatch、render、permission 等运行链路或治理仓库包。

`server/tests/architecture` 强制：

- `TestManagementPackagesDoNotLeakIntoDomainPackages`：`internal/app` 与 `internal/management` 之外的生产代码不得 import `internal/management/*`。
- `TestCompositionRootLayering`：composition root 子包遵守单向装配顺序。
- `TestPluginStackDoesNotImportEventRenderOrGovernanceWiring`：`app/pluginstack` 保持插件状态边界。
- `TestDomainPackagesDoNotImportApp`：领域包不得 import composition root。

## 验证入口

```bash
cd server
go test ./...
mkdir -p dist && go build -o "dist/raylea-server$(go env GOEXE)" ./cmd/raylea-server
```
