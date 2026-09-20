# 通用插件服务调用者

本示例通过 `event.Actions().CallService` 调用 [提供者](../example-service-provider/README.md)，不启动额外 HTTP 服务。默认目标由配置 `provider` 指定。

```go
var result map[string]any
err := event.Actions().CallService(ctx, rayleabot.ServiceCallRequest{
    TargetPluginID: "example-service-provider",
    Service: "resource",
    ServiceVersion: 1,
    Method: "query",
    Params: map[string]any{"id": "item-1"},
}, &result)
```

调用结果仍属于当前事件；调用者负责把业务结果展示给用户。正常命令为 `服务调用示例`。

调用者将最后请求的资源 ID 保存在自己的 KV；提供者保存处理记录，两者使用各自命名空间。调用者也导出一个简单的 `resource.query` 服务，演示同一插件可以同时提供和调用服务；服务处理器本身不继续发起跨插件调用。

集成测试通过现有 `management.action` 传入 `method`、对象 `params` 和可选 `timeout_ms`，验证失败与取消。context 到期时 SDK 会发出服务取消通知；宿主不自动重试调用。
