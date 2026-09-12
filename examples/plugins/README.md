# Plugin Examples

本目录收录与 `contracts/plugin-info.schema.json` 和
`contracts/plugin-protocol.schema.json` 对齐的示例插件。

规则：

- 示例用于理解 `info.json`、插件协议、SDK 入口和常用 local action。
- 每个示例都是独立 Go module：`cmd/<name>/main.go` 是进程入口，示例资源放在顶层目录（如 `templates/`、`ui/`、`assets/`），并通过 `sdk/go` 消费协议客户端与 local action helper；示例保持精简布局，不代表独立插件仓库的完整目录约定。
- 示例后端使用 Go 与 `sdk/go`；插件运行时只要求目标平台原生可执行文件，其他语言实现先生成原生入口再用 `raylea-plugin pack` 打包。
