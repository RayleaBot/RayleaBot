# hello-go

这是与 `contracts/plugin-info.schema.json`、`contracts/plugin-artifact.schema.json` 和 `contracts/plugin-protocol.schema.json` 对齐的最小 Go 插件示例。

示例展示：

- manifest v3 如何声明事件、权限和命令；
- 如何使用 `rayleabot.Run` 注册事件处理器；
- 如何通过 `EventContext.Result` 返回一次终态响应；
- 如何使用统一 `raylea-plugin build-go` 生成按平台分包的 artifact。

在仓库根目录构建 Windows x64 包：

```powershell
go run ./sdk/go/cmd/raylea-plugin build-go --plugin ./examples/plugins/hello-go --backend ./cmd/hello-go --target windows-x64 --out dist/plugin-artifacts
```

服务端只安装构建后的 ZIP 或 artifact 目录，不读取本目录中的 Go 源码。
