# Server Structure Baseline

本文档记录 `server/` 的结构边界、包规模约束和验证入口。

## 结构边界

| 路径 | 职责 |
| --- | --- |
| `server/internal/app` | 应用生命周期、关闭流程、运行状态和 composition root |
| `server/internal/app/platform` | 配置加载后的基础设施组装，包括 SQLite、auth、secret、tasks、scheduler、logging、console |
| `server/internal/app/pluginstack` | 插件主链组装，包括 catalog、adapter、bridge、dispatcher、render、plugin repository、grant repository |
| `server/internal/app/servicegraph` | 应用服务组装，包括 local actions、runtime registry、system、plugin lifecycle、event ingress、protocol、webhook、governance、third-party、Bilibili source |
| `server/internal/app/httpwire` | HTTP server、management handlers、WebSocket handlers 和路由模块组装 |
| `server/internal/management/router` | 管理面公共路由、受保护路由、管理 UI fallback 和健康检查 |
| `server/internal/management/*api` | 各管理面 HTTP API 子域的 handler、request、response 和路由注册 |
| `server/internal/management/pluginapi/view` | 插件管理 API 的列表、详情、授权和 dead-letter response 投影 |
| `server/internal/management/ws` | 管理面 WebSocket 入口 |
| `server/internal/management/events` | 管理面 WebSocket event frame 与 payload 投影 |
| `server/internal/plugins/actions` | 插件 local action registry 和公共入口 |
| `server/internal/plugins/actions/*action` | 单类 local action 的执行逻辑和依赖接口 |
| `server/internal/integrations/bilibili/source` | Bilibili source 编排入口 |
| `server/internal/integrations/bilibili/session/identity` | Bilibili 请求身份、UA、语言和请求头生成 |
| `server/internal/integrations/bilibili/{session,live,dynamic,monitoring,diagnostics,media,credential,accountusage,subscriptions,values}` | Bilibili source 的独立协作者 |
| `server/internal/logging/repository` | 管理日志 SQLite 持久化和查询实现 |
| `server/internal/system/startup` | 启动运行环境阶段、标签、日志字段和失败归因 |
| `server/internal/bot/adapter/onebot11/shell/backoff` | OneBot11 shell 重连退避算法 |

## 包规模约束

`server/tests/architecture` 维护结构回归检查：

| 约束 | 上限 |
| --- | ---: |
| 单目录生产 Go 文件数 | 19 |
| 单目录测试 Go 文件数 | 20 |
| 单个生产 Go 文件行数 | 600 |

生成物、测试工具和构建产物不承担业务边界职责。生产包超出上限时，需要拆出明确职责包；同一职责被拆成多个小文件时，可以合并到同一文件。

## 依赖方向

- `contracts/` 是 HTTP、WebSocket、schema、错误码和发布元数据的正式来源。
- `management/*api` 只承担管理面入口和 DTO 投影，不作为领域服务模型来源。
- `server/internal/app` 可以依赖多个子系统；其它生产包不得通过管理面 API 类型表达业务状态。
- `server/internal/app/httpwire` 负责把运行时配置投影为管理面 handler 所需配置。
- `auth` 持有认证基础设施；管理面登录 handler 只消费认证接口和登录失败计数接口。
- `plugins/actions` 通过 registry 分发 local action；每个 action 子包只持有自身需要的依赖。

`server/tests/architecture` 禁止 `server/internal/app` 与 `server/internal/management` 之外的生产代码 import `server/internal/management/*`。

## 规模边界目录

| 目录 | 生产 Go 文件数 |
| --- | ---: |
| `server/internal/render/service` | 19 |
| `server/internal/plugins/runtime/manager` | 19 |
| `server/internal/bot/adapter/onebot11/shell` | 19 |
| `server/internal/dispatch` | 18 |
| `server/internal/system` | 18 |
| `server/internal/integrations/bilibili/session` | 18 |
| `server/internal/plugins/install` | 18 |

这些目录接近包规模上限。新增职责进入这些目录时，需要优先拆出子包或迁入已有职责包。

## 验证入口

```bash
cd server
go test ./...
mkdir -p dist && go build -o "dist/raylea-server$(go env GOEXE)" ./cmd/raylea-server
```
