# RayleaBot 代码评审与改进清单

本文档记录代码库中已确认的工程问题、影响分析与改进建议。
工程基线、契约与目录职责以 `docs/engineering/baseline.md`、`contracts/` 为准。

---

## 1. 稳定性

### 1.1 生产路径 panic

`server/internal/pluginhttp/client.go:382` 的 `mustParsePrefixes()` 函数在 CIDR 解析失败时调用 `panic`：

```go
panic(fmt.Sprintf("parse bogon prefix %q: %v", value, err))
```

该函数在 Client 初始化路径执行。如果 bogon 前缀列表包含畸形条目，进程直接崩溃。

**建议**：改为显式错误返回，调用方处理初始化失败。

### 1.2 Request ID 熵不足

`server/internal/httpapi/httpapi.go:110` 的 `newRequestID()` 使用 8 字节（64 位）随机数，`rand.Read` 失败时回退到固定值 `"req_0000000000000000"`：

```go
bytes := make([]byte, 8)
if _, err := rand.Read(bytes); err != nil {
    return "req_0000000000000000"
}
return "req_" + hex.EncodeToString(bytes)
```

64 位在高并发场景下碰撞概率可接受，但固定回退值会导致多条日志共享同一 request ID，丧失关联能力。

**建议**：将随机字节长度提升至 16 字节（128 位）。`rand.Read` 在 Go 1.25 实际上不会返回 error，可移除 fallback 分支或改为 `crypto/rand` 保证。

### 1.3 App 构造期资源泄漏

`server/internal/app/app_build_platform.go` 的构造链约 120 行，依次创建 storageStore、authRepository、secretStore、authManager、taskRepository、logRepository、schedulerEngine 等资源。所有失败路径仅关闭 `storageStore`：

```go
// 每个错误分支都只有：
_ = storageStore.Close()
return appPlatform{}, fmt.Errorf(...)
```

如果 `secretStore` 创建成功后在后续步骤失败，其内部资源（如数据库事务句柄）不会被清理。

**建议**：引入清理栈（`[]func()`），按创建顺序入栈、逆序执行。失败时统一 `defer` 释放所有已成功资源。

### 1.4 Dispatcher Register 锁内阻塞

`server/internal/dispatch/dispatch.go:86-92` 的 `Register` 方法在持有 `mu.Lock()` 的状态下等待旧 worker goroutine 退出：

```go
d.mu.Lock()
defer d.mu.Unlock()

if old, ok := d.slots[pluginID]; ok {
    close(old.queue)
    <-old.done   // 持锁等待 goroutine 退出
}
```

对比 `Deregister` 方法在锁外等待（先 `delete` + `Unlock`，再 `close` + `<-done`），`Register` 的实现可能在 worker 尝试获取读锁时造成死锁。

**建议**：参照 `Deregister` 的模式，将 `close(old.queue)` 和 `<-old.done` 移到释放写锁之后。

---

## 2. 安全基线

### 2.1 HTTP Server 无超时限制

`server/internal/app/app_build_http.go:31-34` 构造 `http.Server` 时未设置超时和头部限制：

```go
server := &http.Server{
    Addr:    listenAddr,
    Handler: router,
}
```

`ReadTimeout`、`WriteTimeout`、`IdleTimeout`、`MaxHeaderBytes` 均为默认零值（无限制），可被慢速连接或恶意大请求消耗服务端资源。

**建议**：设置合理超时（如 `ReadTimeout: 30s`、`WriteTimeout: 60s`、`IdleTimeout: 120s`、`MaxHeaderBytes: 1 << 20`）。渲染预览等长时间路由通过 Hijack 或独立 server 处理。

### 2.2 Web 缺少 CSP

`web/index.html` 无 Content-Security-Policy 声明。管理面 XSS 攻击可能导致 session token 泄露或管理操作被劫持。

**建议**：在 `<head>` 中添加 CSP meta 标签，至少限制 `script-src`、`style-src`、`connect-src` 为同源。Vite 开发模式需额外允许 HMR WebSocket。

### 2.3 前端 HTTP 无超时

`web/src/lib/http.ts` 的两处 `fetch` 调用均无 `AbortController` 超时控制：

```typescript
const response = await fetch(path, {
    ...rest,
    headers: requestHeaders,
    body: body === undefined ? undefined : JSON.stringify(body),
})
```

网络异常或服务端无响应时，请求无限挂起。

**建议**：封装默认超时（如 30 秒），通过 `AbortController` + `setTimeout` 实现，允许调用方传入自定义 `signal` 覆盖。

---

## 3. 存储层

### 3.1 SQLite PRAGMA 拼接

`server/internal/storage/store.go:157` 使用 `fmt.Sprintf` 拼接 PRAGMA 语句：

```go
db.ExecContext(ctx, fmt.Sprintf("PRAGMA busy_timeout = %d", busyTimeout.Milliseconds()))
```

`busyTimeout.Milliseconds()` 返回 `int64`，`%d` 格式化不存在注入风险，但其他 PRAGMA 使用参数化查询（如 `PRAGMA foreign_keys = ON` 直接写常量字符串），风格不一致。

**建议**：统一为字符串常量或参数化写法，保持 PRAGMA 配置的一致性。

### 3.2 读连接未声明只读

`configureHandle()` 对读写两个连接执行相同的配置流程，读连接未在 DSN 中声明 `mode=ro` 或在连接级别约束只读。误用读连接执行写操作时不会被数据库层拒绝。

**建议**：为读连接 DSN 添加 `?mode=ro` 参数，或在 `configureHandle()` 中根据连接角色区分配置。

---

## 4. 消息与 i18n

Server 侧部分 HTTP handler 中嵌入了自然语言字符串作为用户可见错误消息（如 `server/internal/httpapi/httpapi.go` 和 `server/internal/plugins/http.go` 中的 `"内部错误"` 等中文文本）。

`ErrorEnvelope` 结构已包含 `code`（错误码）和 `message_key`（消息键）字段，但部分路径仅填写了 `message` 而未填写 `message_key`。Web 侧 `ApiError` 已支持 `messageKey` 使用，i18n 体系（`web/src/locales/zh-CN.ts`）已就绪。

**建议**：需要用户可见消息的路径统一填写 `message_key`，Web 侧优先展示 `message_key` 对应的本地化文本，`message` 作为 fallback。

---

## 5. 维护性

### 5.1 API 类型集中文件

`web/src/types/api.ts`（391 行）承载全部 API 类型定义，包含 11 种 TaskType、多种枚举、所有请求/响应接口和 WebSocket 帧定义。随着路由增长（`contracts/web-api.openapi.yaml` 已定义 26 个路由），该文件的修改频率和合并冲突概率会持续上升。

**建议**：按领域拆分类型文件（如 `types/tasks.ts`、`types/plugins.ts`、`types/system.ts`），或引入 `openapi-typescript` 从契约自动生成（参见 `docs/engineering/tech-stack-evaluation.md`）。

### 5.2 规划文档体量

`docs/RayleaBot机器人项目规划.md` 约 3426 行。作为单一文档，跨章节检索和并行编辑成本高。

**建议**：按主题拆分为独立文档，保留主文件作为目录索引。

### 5.3 i18n 文案文件

`web/src/locales/zh-CN.ts`（461 行）按扁平结构组织所有页面的中文文案。功能扩展后文件体量和键名冲突风险上升。

**建议**：按页面或功能域拆分为独立文件，通过 `vue-i18n` 的 lazy loading 按需加载。

---

## 6. CI 门禁覆盖

当前 `lint.yml` 门禁包含：

| 检查项 | 覆盖面 |
|--------|--------|
| `go test ./...` | Server 全量单元/集成测试 |
| `go build ./cmd/raylea-server` | Server 编译验证 |
| `golangci-lint`（errcheck, staticcheck, govet, unused, ineffassign） | Go 静态分析 |
| Go 覆盖率门禁（阈值 55%） | Server 测试覆盖率 |
| `govulncheck` | Go 依赖漏洞扫描 |
| `pnpm run typecheck` + `pnpm test:coverage` + `pnpm build` | Web 类型检查、测试覆盖率、构建验证 |
| `pnpm run typecheck` + `pnpm test:coverage` + `pnpm build` | Launcher 类型检查、测试覆盖率、构建验证 |

`go test -race ./...` 作为独立工作流（`race.yml`）保留，按需手动触发。

---

## 7. 优先级

| 等级 | 改进项 | 影响 |
|------|--------|------|
| P0 | 移除 `pluginhttp/client.go` 生产路径 `panic` | 避免进程崩溃 |
| P0 | 为 `http.Server` 设置超时和 `MaxHeaderBytes` | 防御资源耗尽攻击 |
| P0 | 修复 `Register` 锁内阻塞 | 消除死锁风险 |
| P1 | App 构造期引入资源清理栈 | 防止构造失败时资源泄漏 |
| P1 | 前端 HTTP 封装增加超时与取消 | 防止请求无限挂起 |
| P1 | Web 入口添加 CSP | 降低 XSS 风险 |
| P1 | 提升 request ID 到 128 位 | 增强日志关联能力 |
| P2 | 统一 Server 侧 `message_key` 填写 | 支持前端 i18n 展示 |
| P2 | SQLite 读连接声明只读 | 防御误写 |
| P2 | 拆分 `api.ts` / i18n 文案 / 规划文档 | 降低维护冲突 |
