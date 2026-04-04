# RayleaBot 代码评审与改进方向

本文档记录对 RayleaBot 仓库的软件工程评审结论与持续改进方向，供核心贡献者在后续迭代中参考。

---

## 1. 架构与代码组织

### 1.1 核心结构体过度集中

`server/internal/app/app.go` 中的 `App` struct 持有近 40 个字段，涵盖生命周期管理、路由组装、业务编排、状态缓存和各类 repository。这种"上帝对象"模式导致：

- 并发开发时冲突率高
- 单元测试难以拆分和 mock
- 新成员理解成本大

**改进方向**：将 HTTP 路由注册、插件生命周期、本地 action 执行、任务调度、启动时序等职责拆分为独立的 `Coordinator` 或 `Facade`，`App` 仅保留依赖注入与启动/停止编排。

### 1.2 单文件行数过大

以下文件同时封装了数据模型、状态机、I/O 逻辑和业务规则，已超出健康维护阈值：

| 文件 | 行数 |
|------|------|
| `server/internal/runtime/manager.go` | ~2044 |
| `server/internal/app/app.go` | ~1107 |
| `server/internal/plugins/install.go` | ~926 |
| `launcher/src/renderer/src/AppShell.tsx` | ~869 |
| `web/src/pages/DashboardPage.vue` | ~664 |

**改进方向**：按行为拆分超大文件。例如 `runtime/manager.go` 可拆为 `process.go`（子进程管理）、`protocol.go`（JSONL 读写）、`state_machine.go`（状态流转）、`action_handler.go`（action 分发），单文件控制在 400 行以内。

### 1.3 前端技术栈不统一

Web 管理面使用 Vue 3 + Element Plus，Launcher 渲染层使用 React 18 + Fluent UI。两套组件系统、两套状态管理范式和两套设计 token 增加了维护与视觉一致性保障成本。

**改进方向**：长期评估将 Web 管理面以 WebView 嵌入 Launcher，或将 Launcher 收敛为"系统托盘 + 窗口壳"，全部交互逻辑保留在 Vue 侧，逐步消除 React 技术栈。

---

## 2. 测试与质量保障

### 2.1 缺少代码覆盖率门禁

CI 运行 `go test ./...`、`pnpm test`、`pnpm test:e2e`，但未收集或上报代码覆盖率，也没有设定阈值。

**改进方向**：
- Go 侧：`go test -coverprofile` + `codecov-action` 或等效方案
- Web / Launcher 侧：Vitest 开启 `coverage`，为核心模块设置阈值（建议 ≥70%）

### 2.2 测试分布不均

- Go 侧：`runtime/manager_test.go` 独占约 2000 行，而 `adapter`、`render`、`bridge` 等关键模块的单元测试密度明显不足
- Web 侧：4167 个 TS/Vue 文件仅有 25 个 `.spec.ts`，且以 Playwright E2E 为主，单元测试密度偏低

**改进方向**：为 `adapter`、`render`、`bridge`、`permission` 增加 table-driven 单元测试；为 Web 侧的 store、utils、HTTP 封装层补 Vitest 单元测试，降低对 E2E 的路径依赖。

### 2.3 属性测试未常态化

`go.mod` 已引入 `pgregory.net/rapid`，但仅在个别模块使用，未在协议解析、配置校验、事件序列化等边界形成常态化属性测试。

**改进方向**：在 `plugin-protocol` parser、`config` canonicalization、`command` parser 等关键边界引入 property-based testing，作为 fixtures 的互补验证手段。

### 2.4 Race Detector 因 CGO 禁用无法运行

由于项目使用 `modernc.org/sqlite`（纯 Go 实现），CGO 默认被关闭，导致 `go test -race` 报错 "requires cgo"。当前 CI 未运行 race detector，无法自动发现数据竞态。

**改进方向**：在 CI 的 `server-smoke` job 中显式设置 `CGO_ENABLED=1` 并运行 `go test -race`，或定期在本地/手动 workflow 中执行全量竞态检测。

---

## 3. CI/CD 与工程实践

### 3.1 静态分析工具链缺失

仓库中没有 `.golangci.yml`、`.eslintrc`、`.prettierrc` 或 `biome.json`。`lint.yml` 仅校验 baseline 存在性和 contracts 结构，未对 Go/TS/Vue/TSX 代码执行静态质量扫描。

**改进方向**：
- Go：`golangci-lint`，至少启用 `errcheck`、`staticcheck`、`govet`、`unused`、`ineffassign`
- Web / Launcher：`eslint` + `prettier` 或统一使用 `biome`

### 3.2 Release Workflow 重复度高

`.github/workflows/release.yml` 中的 `build-windows-full`、`build-linux`、`build-macos-full` 三 job 的构建步骤高度雷同，仅在平台相关的 binary 名称和打包路径上存在差异。

**改进方向**：提取公共构建步骤为 reusable workflow 或 composite action，利用 matrix 收敛平台差异，减少三处同步修改的风险。

### 3.3 缺少依赖安全扫描

CI 中未启用 `govulncheck`、`npm audit` 或等效工具，无法自动发现 `go.mod` 和 `pnpm-lock.yaml` 中的已知 CVE。

**改进方向**：在 PR 与主分支 CI 中加入 `govulncheck ./...` 和 `pnpm audit --audit-level moderate`。

---

## 4. 代码质量

### 4.1 生产路径存在 panic

`server/internal/pluginhttp/client.go:382` 在解析 bogon IP 前缀时使用了 `panic(...)`。panic 不应出现在可恢复的网络/配置错误路径中。

**改进方向**：将该 panic 替换为返回可导出的错误类型（如 `ErrInvalidBogonPrefix`），由调用方决定降级或上报策略。

### 4.2 错误处理不统一

仓库中 `fmt.Errorf("...%w", err)` 有 240 处，而 `errors.New(...)` 仅 56 处，且缺少统一的错误类型体系。大量错误以裸字符串形式散落，不利于调用方做 `errors.Is` / `errors.As` 判定。

**改进方向**：在 `server/internal/errcode` 或 `server/pkg/errors` 中定义 `ValidationError`、`IOError`、`TimeoutError` 等类型，统一错误包装方式，减少裸字符串错误。

### 4.3 魔法数字分散

部分常量已提取（如 `defaultReplyTargetCacheSize = 10000`、`maxRemoteDownloadBytes = 256 MB`），但 adapter 重试参数、render 并发数、HTTP timeout 等仍以字面量形式出现在业务逻辑中。

**改进方向**：将所有超时、重试、并发上限、缓冲区大小收敛到 `config/default.yaml` 或 `internal/defaults` 常量包中统一管理。

### 4.4 硬编码中文文案散落在业务逻辑中

以下核心模块中出现了未经过 i18n 层的硬编码中文：

- `tasks/executor.go`："任务已取消"、"完成"
- `plugins/install.go`："插件安装超时"

**改进方向**：将用户可见文案统一收口到 `contracts/error-codes.yaml` 或仓库级消息资源中，业务代码只引用消息键，不直接硬编码自然语言文本。

### 4.5 Request ID 熵不足

`server/internal/httpapi/httpapi.go` 中的 `newRequestID()` 仅使用 8 字节随机数，且在 `rand.Read` 失败时回退到固定字符串 `"req_0000000000000000"`。高并发场景下可能出现 request ID 冲突，影响日志追踪与排障。

**改进方向**：使用至少 16 字节随机数，或结合时间戳与随机数生成 request ID；移除固定字符串回退，失败时改用 `crypto/rand` 的替代方案或 panic（仅在启动时）。

---

## 5. 文档与治理

### 5.1 规划文档体量过大

`docs/RayleaBot机器人项目规划.md` 超过 1000 行，涵盖架构、协议映射、状态机、权限模型、API 细节等。任何契约或实现微调都需要同步修改大篇幅 Markdown，维护成本高，容易与 `contracts/` 产生口径漂移。

**改进方向**：按领域拆分为 5-8 个独立小册，例如 `architecture/overview.md`、`plugin-protocol/reference.md`、`user-guide/deployment.md` 等，每个文件聚焦单一子系统。

### 5.2 AGENTS.md 规则未工具化

根目录 `AGENTS.md` 包含大量提交规范、文档措辞禁忌、阶段边界说明，但其中可自动校验的内容（如 Conventional Commits 类型、禁用词列表）仍依赖人工审查。

**改进方向**：
- 用自定义脚本或 `markdownlint` 插件检查禁用词（如"不再"、"已改为"、"阶段1"）
- CI 中加入 `commitlint`，自动校验 Conventional Commits 格式
- 用脚本检查 Markdown 中是否出现了 `contracts/` 中已废弃的字段名

---

## 6. 依赖管理

### 6.1 Go module path 非正式

`server/go.mod` 使用 `module rayleabot/server`，`docs/engineering/baseline.md` 中标记为 `TODO(repo.identity)`。外部贡献者或 CI 在特定 GOPROXY 环境下可能遇到解析问题。

**改进方向**：在仓库配置正式 remote 后，将 `go.mod` 更新为 `github.com/rayleabot/rayleabot/server`（或最终确定的正式路径），并同步移除 baseline 中的 TODO。

### 6.2 测试依赖混编

`pgregory.net/rapid` 作为测试专用库出现在 `go.mod` 的主 `require` 段，增加了生产构建的依赖解析面。

**改进方向**：将该库移到 test-only require 块，或在构建生产二进制时利用 Go 工具链的 test 依赖隔离能力减少其影响。

### 6.3 前端依赖版本前缀风险

Web 的 `package.json` 中部分依赖使用 `^` 前缀（如 `vue-i18n ^11.3.0`、`lucide-vue-next ^0.511.0`），在未使用 `--frozen-lockfile` 时存在意外升级窗口。

**改进方向**：确保 CI 和本地开发脚本中的 `pnpm install` 始终携带 `--frozen-lockfile`；在 PR 中检查 lockfile 是否与 `package.json` 同步。

---

## 7. 前端工程

### 7.1 Pinia stores 职责边界模糊

`web/src/stores/` 下 7+ 个 store 同时持有 UI 状态与远程数据缓存，跨 store 依赖关系不清晰。

**改进方向**：将远程数据缓存与本地 UI 状态拆分为两类 store。例如 `pluginsData.ts` 负责 API 数据缓存，`pluginsUI.ts` 负责展开、选中、过滤条件等本地状态。

### 7.2 类型定义集中度过高

`web/src/types/api.ts` 约 412 行，集中了全部 API 类型。任何契约字段的增删都会影响全量类型文件，合并冲突率高。

**改进方向**：按 OpenAPI tag 拆分为 `types/api/auth.ts`、`types/api/plugins.ts`、`types/api/tasks.ts` 等。

### 7.3 i18n 文件过大

`web/src/locales/zh-CN.ts` 约 447 行，包含大量带插值变量的长文案，且未按页面拆分。

**改进方向**：按页面拆分为 `locales/zh-CN/pages/dashboard.json`、`locales/zh-CN/pages/plugins.json` 等，通过 `vue-i18n` 命名空间加载，降低维护门槛并为后续多语言扩展做准备。

### 7.4 `fetch` 缺少超时与取消机制

`web/src/lib/http.ts` 中的 `apiRequest` 直接使用裸 `fetch`，未配置 `AbortSignal` 或请求超时。在网络异常或后端挂起时，HTTP 请求可能无限等待，导致 UI 按钮长时间处于加载态。

**改进方向**：为 `apiRequest` 封装 `AbortController`，设置默认请求超时（如 30 秒），并在组件卸载或路由切换时主动取消未完成的请求。

---

## 8. 安全与生产就绪

### 8.1 HTTP Server 缺少超时与安全头

`server/internal/app/app.go` 在构造 `http.Server` 时仅设置了 `Addr` 与 `Handler`，未配置：

- `ReadTimeout` / `WriteTimeout` / `IdleTimeout`
- `MaxHeaderBytes`
- 任何安全中间件（CORS、X-Frame-Options、X-Content-Type-Options、Content-Security-Policy）

缺少超时配置会使服务容易受到慢loris攻击或连接耗尽；缺少安全头则增加了 XSS、点击劫持等风险面。

**改进方向**：
- 为 `http.Server` 设置合理的读写超时与头部大小限制
- 添加基础安全中间件，至少包含 `X-Content-Type-Options: nosniff`、`X-Frame-Options: DENY`、CSP 白名单

### 8.2 前端缺少 Content Security Policy

`web/index.html` 未设置任何 CSP `<meta>` 标签。由于 Web 管理面需要处理插件安装、系统配置等敏感操作，缺少 CSP 会降低对 XSS 注入的防御纵深。

**改进方向**：在 `web/index.html` 和 Launcher 的窗口加载逻辑中加入严格的 CSP 策略，限制脚本、样式和连接的合法来源。

---

## 9. 并发与资源管理

### 9.1 初始化失败导致资源泄漏

`server/internal/app/app.go` 的 `New` 函数包含近 30 步连续初始化。若中间任何一步返回错误，代码仅执行 `_ = storageStore.Close()`，而此前已创建的 `consoleStream`、`taskExecutor`、`adapterShell`、`renderService`、`eventDispatcher`、`eventBridge`、`schedulerEngine` 等资源均未被清理。

这些组件中部分已启动后台 goroutine（如日志流、任务执行器、调度器），未清理将造成 goroutine 泄漏、文件句柄泄漏和内存泄漏。

**改进方向**：引入构造期资源追踪器（如 `io.Closer` 切片），每一步成功后将可关闭对象入栈，失败时按 LIFO 顺序统一清理。

### 9.2 `Dispatcher.Register` 存在阻塞风险

`server/internal/dispatch/dispatch.go` 的 `Register` 方法在持有 `d.mu` 写锁的情况下，执行 `close(old.queue)` 后紧接着等待 `<-old.done`。如果旧 worker 因 `DeliverEvent` 阻塞或陷入死循环，`<-old.done` 将永远等待，导致 dispatcher 的所有读写操作被死锁。

**改进方向**：将 `<-old.done` 移到锁外，或为等待操作添加超时机制；在 reload 场景下使用带超时的 graceful shutdown。

### 9.3 SQLite 配置存在改进空间

`server/internal/storage/store.go` 存在以下问题：

- 使用 `fmt.Sprintf` 拼接 `PRAGMA busy_timeout` SQL
- read handle 未配置为只读模式，理论上仍可进行写入操作
- 缺少 `PRAGMA synchronous` 与 `PRAGMA temp_store` 的生产环境建议配置

**改进方向**：使用参数化方式设置 PRAGMA（如 `db.Exec("PRAGMA busy_timeout = ?", ms)`，若驱动支持）；为 read handle 附加 `_query_only=1` 或 `mode=ro`；显式配置 `synchronous` 与 `temp_store` 以匹配预期的持久化与性能要求。

---

## 优先级建议

| 优先级 | 改进项 | 预期收益 |
|--------|--------|----------|
| P0 | 移除生产代码中的 panic；拆分 `runtime/manager.go` 与 `App` struct | 提升稳定性与可维护性 |
| P0 | CI 接入 `golangci-lint`、`eslint` 与 `govulncheck` | 建立自动化质量与安全门禁 |
| P0 | 为 HTTP Server 配置超时与安全头；修复 `New` 函数资源泄漏 | 提升生产安全性与健壮性 |
| P1 | 引入代码覆盖率报告与阈值 | 量化测试完整性，防止回归 |
| P1 | 重构 `release.yml` 使用 matrix / reusable workflow | 降低 CI 维护成本 |
| P1 | CI 启用 race detector（`CGO_ENABLED=1`） | 主动发现数据竞态 |
| P2 | 拆分规划文档与类型定义文件 | 降低文档同步成本与合并冲突 |
| P2 | 收敛 Go module path；分离 test-only 依赖 | 提升工程专业度与构建可复现性 |
| P2 | 前端 `fetch` 增加超时与取消机制；配置 CSP | 提升 Web 管理面健壮性与安全纵深 |
