# Server Structure Baseline

本文档记录 `server/` 的结构边界、依赖方向和验证入口。

## 结构边界

| 路径 | 职责 |
| --- | --- |
| `server/internal/app` | 应用生命周期、关闭流程、运行状态、基础设施、插件状态、事件管线和应用服务组装 |
| `server/internal/management` | 管理面 HTTP API、handler、request、response、路由注册和插件管理 API 投影 |
| `server/internal/plugins/actions` | 插件 local action registry 和公共入口 |
| `server/internal/integrations/bilibili/session` | Bilibili 登录、扫码、风控校验和账号资料读取 |
| `server/internal/integrations/fingerprint` | 浏览器指纹算法 |
| `server/internal/logging` | 管理日志流、SQLite 持久化和查询实现 |
| `server/internal/system` | 运行环境准备、状态、诊断、任务和失败归因 |
| `server/internal/onebot11` | OneBot11 shell、协议模型、收发消息、缓存和重连退避 |

## 包职责原则

职责边界由**依赖方向**和**类型内聚**表达，不由文件数或行数表达。

- 一个类型的方法散落到多个文件，不构成职责边界。包内职责过重时，按职责抽出**协作者类型**或**子包**（各自持有自身状态与测试），让原类型退化为协调器；不要按方法前缀把同一类型切成更多文件。
- 同一职责被拆成多个微文件（例如把 handler 与其专用 request/response 类型拆成 `_handlers.go` + `_types.go`）时，合并回同一文件。
- 生成物、测试工具和构建产物不承担业务边界职责。

`server/tests/architecture` 维护结构回归检查。其中行数检查仅作为拦截病态文件的宽松安全网（单个生产 Go 文件 ≤ 1500 行），不作为拆分驱动指标。

## 依赖方向

- `contracts/` 是 HTTP、WebSocket、schema、错误码和发布元数据的正式来源。
- `management` 只承担管理面入口和 DTO 投影，不作为领域服务模型来源。
- composition root 单向装配：基础设施 → render wiring → service wiring → HTTP wiring。领域包不得反向 import `internal/app`。
- 领域包不得 import composition root：除入口/装配层（`internal/app`）和 `server/tests/**` 外，`internal/` 下生产代码不得 import `internal/app`。
- `server/internal/app` 负责把运行时配置投影为管理面 handler 所需配置。
- `auth` 持有认证基础设施；管理面登录 handler 只消费认证接口和登录失败计数接口。
- `plugins/actions` 通过 registry 分发 local action；每个 action 文件只持有自身需要的依赖。
- `server/internal/app` 的插件状态组装只承接 catalog、插件仓库和插件存储对象；运行链路和治理仓库通过服务组装函数注入。

`server/tests/architecture` 强制：

- `TestManagementPackagesDoNotLeakIntoDomainPackages`：`internal/app` 与 `internal/management` 之外的生产代码不得 import `internal/management/*`。
- `TestRenderImplementationPackagesStayBehindServiceBoundary`：render implementation 保持在 service 边界后。
- `TestDomainPackagesDoNotImportApp`：领域包不得 import composition root。
- `TestProductionFilesStayReadable`：生产 Go 文件保持可读规模。
- `TestTestFilesUseScenarioNames`：测试文件名使用场景名，避免编号分片。

## 验证入口

```bash
cd server
go test ./...
mkdir -p dist && go build -o "dist/raylea-server$(go env GOEXE)" ./cmd/raylea-server
```
