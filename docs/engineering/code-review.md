# RayleaBot 代码评审结论与改进清单

本文档聚焦当前仓库状态下已经证实的工程问题、实际影响与可执行改进建议。
基线、契约与正式目录职责继续以 `docs/engineering/baseline.md`、`docs/engineering/implementation-order.md` 和 `contracts/` 为准。

当前仓库的 `App`、`runtime`、插件安装、Launcher 壳层和系统状态页采用职责拆分结构。以下问题列表继续跟踪稳定性、安全基线和长期维护性。

---

## 1. 当前确认的问题

### 1.1 生产路径稳定性

以下问题已经可以直接在当前代码中定位：

- `server/internal/pluginhttp/client.go` 继续存在生产路径 `panic`
- `server/internal/httpapi/httpapi.go` 中 `newRequestID()` 仅使用 8 字节随机数，失败时回退固定值
- `server/internal/app/app.go` 的构造链很长，失败路径主要只关闭 `storageStore`
- `server/internal/dispatch/dispatch.go` 的 `Register` 在锁内等待旧 worker 退出

这些问题会直接影响稳定性、排障能力和极端情况下的服务可用性。

**建议**

- 将生产路径 `panic` 改为显式错误返回。
- 提高 request ID 熵，移除固定回退值。
- 为构造期资源引入统一清理栈。
- 将 dispatcher 的等待行为移到锁外，或加入明确超时控制。

### 1.2 存储、网络与安全基线

当前仍有几项明确的工程缺口：

- `server/internal/storage/store.go` 继续使用 `fmt.Sprintf` 拼接 `PRAGMA busy_timeout`
- 读连接未声明只读语义
- `http.Server` 构造处仍未设置 `ReadTimeout`、`WriteTimeout`、`IdleTimeout`、`MaxHeaderBytes`
- `web/index.html` 仍无 CSP
- `web/src/lib/http.ts` 继续使用裸 `fetch`，没有超时和取消机制

**建议**

- 收紧 SQLite 连接配置，明确读连接语义和常用生产参数。
- 为 `http.Server` 配置超时和头部限制，并补基础安全头。
- 为 Web 与 Launcher 入口增加 CSP 策略。
- 为前端 HTTP 封装增加默认超时与取消能力。

### 1.3 文档、类型与前端维护性

以下问题属于维护性观察项，已经有足够证据支持继续关注：

- `docs/RayleaBot机器人项目规划.md` 当前约 `4282` 行
- `web/src/locales/zh-CN.ts` 当前约 `461` 行
- `web/src/types/api.ts` 集中承载全部 API 类型
- `web/src/stores/` 当前有 `7` 个 store，远程数据与页面状态混放的情况仍然存在

这些问题会提高合并冲突率和局部修改成本，但影响级别低于稳定性、安全性与 CI 门禁问题。

**建议**

- 按领域拆分超大规划文档。
- 按页面或领域拆分前端类型与 i18n 文案。
- 逐步分离前端远程数据缓存与页面本地状态。

### 1.4 用户可见消息边界

用户可见自然语言消息仍然散落在部分业务代码中。当前问题的重点是消息是否绕过了正式错误码或消息键边界，不是中文本身。

当某条消息已经存在正式错误码、消息键或统一消息面时，业务代码直接写自然语言会增加跨端一致性维护成本。

**建议**

- 继续将可归入正式错误面和消息面的文本收口到统一位置。
- 业务代码优先引用消息键、错误码或统一文案入口。

---

## 2. 当前基线中的已固定事实

以下内容属于仓库已经明确固定的现状，不作为本评审文档中的否定对象：

- Web 管理面固定使用 Vue；Launcher 渲染层固定使用 React
- `.deps/manifest.json` 已采用 v3 结构，运行环境来源使用有序 `sources`
- `recovery.recheck`、`recovery.confirm`、`runtime.bootstrap` 已是正式操作入口
- 前端默认安装命令已固定为 `pnpm install --frozen-lockfile`
- Go、Web、Launcher 已有覆盖率采集与最低门槛
- `lint.yml` 默认门禁当前包含 `golangci-lint`、Go/Web/Launcher Linux 核心静态与覆盖率检查、`govulncheck`，生产依赖审计保留在手动回归路径
- `go test -race ./...` 保留为独立手动回归路径
- `release.yml` 已采用 matrix 构建全量桌面包，并保留独立的 Linux server 制品
- 第一批 store、renderer hook、配置规范化与命令边界测试已进入默认测试集

依赖版本仍有 `^` 前缀的风险，主要通过锁文件和安装门禁控制；这一点属于持续约束，不构成对当前基线本身的否定。

---

## 3. 优先级建议

| 优先级 | 改进项 | 预期收益 |
| --- | --- | --- |
| P0 | 移除生产路径 `panic`、补齐 `http.Server` 超时与头部限制 | 降低服务中断与排障成本 |
| P0 | 修复构造期资源清理与 dispatcher 锁内等待 | 降低泄漏、死锁与停机风险 |
| P1 | 为前端 HTTP 封装增加超时与取消 | 降低前端挂起与长时间加载问题 |
| P1 | 提高 request ID 质量 | 提升日志关联与问题定位能力 |
| P2 | 拆分超大文件与超大文档 | 降低修改冲突与阅读成本 |
| P2 | 拆分前端类型、文案与 store 职责 | 降低前端维护成本 |
| P2 | 细化 SQLite 生产配置 | 提升存储层一致性与可控性 |
