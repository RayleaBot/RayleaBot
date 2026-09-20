# 通用插件服务示例

提供者 `example-service-provider` 和 [调用者](../example-service-consumer/README.md) 使用同一套 JSONL 通道完成跨插件调用。

## 提供服务

在 `info.json` 的 `services` 中声明 `resource` 服务版本 1 和公开方法，再通过 Go SDK 的 `Options.Services` 注册同名处理器：

```go
options := rayleabot.Options{
    Services: []rayleabot.Service{{
        Name: "resource",
        Version: 1,
        Methods: map[string]rayleabot.ServiceHandler{
            "query": func(ctx context.Context, event *rayleabot.EventContext, request rayleabot.ServiceRequest) (map[string]any, error) {
                return map[string]any{"caller": request.CallerPluginID}, nil
            },
        },
    }},
}
err := rayleabot.Run(ctx, options, nil)
```

服务处理器使用自己的事件上下文，示例的 `query` 会把请求 ID 值写入提供者自己的 KV，再返回资源 ID、真实 caller 和来源摘要。其余方法用于展示取消、结构化失败和首版嵌套调用拒绝：

| 方法 | 结果 |
| --- | --- |
| `query` | 使用自身 KV，返回业务对象 |
| `wait` | 等待 context 取消，展示正在执行的服务如何结束 |
| `fail` | 返回带 details 的 `ActionError` |
| `nested` | 在服务中发起下一次服务调用，收到 `plugin.call_chain_rejected` |

普通 Go error 不直接公开正文；可公开的业务错误使用 `ActionError`。调用者或父事件取消后要停止新的业务操作，已经发生的外部副作用不会自动回滚。

## 构建与使用

在仓库根目录构建两个独立 artifact：

```powershell
go run ./sdk/go/cmd/raylea-plugin build-go --plugin ./examples/plugins/example-service-provider --target windows-x64 --out ./dist/service-examples
go run ./sdk/go/cmd/raylea-plugin build-go --plugin ./examples/plugins/example-service-consumer --target windows-x64 --out ./dist/service-examples
```

安装后分别启用两个插件，发送 `服务调用示例`。停用提供者后再次调用，会返回服务不可用，宿主不会自动启动它。

真实进程回归位于 [plugin_services_test.go](../../../server/tests/integration/plugin_services_test.go)，覆盖来源、私有动作、结构化错误、取消、父期限、重载、数据边界及停止状态。
