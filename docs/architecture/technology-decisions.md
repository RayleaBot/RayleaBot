# Technology Decisions

除非现有技术栈无法解决已确认的具体限制，RayleaBot Server 保持当前技术栈。

## 当前基线

| 领域 | 决策 |
| --- | --- |
| Server 语言 | Go |
| HTTP 路由 | chi |
| WebSocket | coder/websocket |
| 存储 | SQLite（modernc.org/sqlite） |
| SQL 访问 | sqlc |
| YAML | gopkg.in/yaml.v3 |
| JSON Schema | santhosh-tekuri/jsonschema v6 |
| 日志 | slog |
| 指标 | Prometheus 兼容 registry |
| 浏览器自动化 | chromedp |
| 媒体处理 | 核心托管的 FFmpeg / FFprobe 二进制，受信本地插件通过固定环境变量复用 |

## 决策规则

新增依赖或替换工具必须记录：

- 需要解决的具体问题；
- 现有技术栈无法解决该问题的原因；
- 是否会引入平行技术栈；
- 回滚路径；
- 对 CI、发布打包、lockfile、fixture 和生成文件的影响。

## 当前评估方向

| 领域 | 当前方向 |
| --- | --- |
| 数据库迁移工具 | 在漂移测试持续有效的前提下，保留当前 schema 快照与 legacy migration runner；只通过小规模 spike 评估 goose、golang-migrate 或 Atlas。 |
| OpenAPI 实现 | 保留严格契约校验和生成类型检查；只有 handler 漂移持续发生时才评估 Server 侧 OpenAPI 代码生成。 |
| Secret 存储 | 保留 SQLite 支持的密封 secret；当部署目标要求外部密钥托管时，再评估环境密钥、操作系统 keychain 或外部 KMS。 |
| 架构门禁 | 保留仓库专用的结构测试和预算文件，因为它们比通用 linter 更准确地表达本仓库包边界。 |
| 媒体处理 | 使用 `.deps/manifest.json` 固定三平台 full GPL FFmpeg / FFprobe 资源，不在各插件内重复打包，也不新增 Go 媒体编解码栈。 |
