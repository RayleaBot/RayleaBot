# hello-go

这是与 `contracts/plugin-info.schema.json`、`contracts/plugin-artifact.schema.json` 和 `contracts/plugin-protocol.schema.json` 对齐的最小 Go 插件示例。

示例展示：

- manifest v2 如何声明 Go 后端、目标平台和能力；
- 如何使用 `rayleabot.Run` 注册事件处理器；
- 如何通过 `EventContext.Result` 返回一次终态响应；
- 如何使用插件自己的 `tools/build` 入口生成按平台分包的 artifact。

在仓库根目录构建 Windows x64 包：

```powershell
go run ./examples/plugins/hello-go/tools/build --target windows-x64 --output dist/plugin-artifacts
```

服务端只安装构建后的 ZIP 或 artifact 目录，不读取本目录中的 Go 源码。
