# Plugin SDK

RayleaBot 插件运行时与实现语言无关。主仓库提供 Go 后端 SDK、通用 artifact 工具和 Vue 管理页 SDK；插件运行时只读取已编译产物。

## Go SDK

`sdk/go` 是独立 Go module：

```go
err := rayleabot.Run(ctx, rayleabot.Options{}, rayleabot.HandlerFunc(
    func(ctx context.Context, event *rayleabot.EventContext) error {
        if event.Bot.ID == "" {
            return event.Result(nil)
        }
        return event.SendText("ok")
    },
))
```

插件 ID、并发度、权限、配置、管理员和命令前缀都来自 init。`EventContext` 提供：

- 当前事件与 request ID。
- 宿主分配的插件 ID。
- 当前 Bot 身份。
- 隔离的完整配置快照。
- 生效权限、超级管理员和命令前缀。
- 宿主当前生效的 `Location`；显示时间使用 `timestamp.In(event.Location)`，动作边界可通过 `event.Actions().TimeLocation()` 取得同一个时区。

每个事件只能发送一次 `Result`、`Fail`、`Send`、`SendText` 或 `Reply` 终态。`Reply` 仍使用 protocol v3 的统一 `message.send` action。

`event.Actions()` 提供 request-bound typed helpers：

- 非终态消息、日志、KV、文件、HTTP、配置写入、插件列表和 secret。
- 治理、scheduler、渲染和三方账号动作。
- OneBot 单动作与 provider 扩展动作。
- 已进入正式 contract 的通用 `Call`。

OneBot 和 provider typed helpers 默认由宿主按当前聊天事件选择实例。定时任务等平台事件需要主动指定实例时，使用 `event.Actions().ForOneBotAdapter("second-bot").GroupInfoGet(ctx, groupID)`；返回的动作视图不会改变其他调用的实例。显式实例仍须与聊天父事件一致，多实例且没有选择信息时宿主拒绝调用。

SDK 串行写 stdout JSONL，日志写 stderr；负责 request 关联、并发、ping/pong、关闭、panic 隔离和配置快照原子替换。

Server 与 Go SDK 的 wire 模型由 `scripts/generate-plugin-wire.py` 从正式协议 schema 生成，分别放在 Server 的 `internal/plugins/pluginwire` 与 SDK 的 `internal/pluginwire` 中。SDK 可用 `GOWORK=off go test ./...` 独立验证，不依赖 Server internal 包。修改契约后运行该生成器和 `node scripts/generate-runtime-schemas.mjs`；两者的 `--verify` 检查缺失、变化和多余的自有生成产物。

生成器只负责传输结构投影：required 字段保留零值，语义需要区分缺省的字段使用指针或 RawMessage，KV 的显式 `null` 不等同于缺少 value。动作 data、动态配置和扩展 payload 保留 JSON 边界。Server 与 SDK 在实际收发时使用内嵌的同一 schema 校验分支、必填字段、整数、未知字段及帧字节上限；动作权限、请求关联和生命周期状态仍由各自运行时检查。`oneOf` 的结构投影不能代替这些校验。

artifact、manifest、运行时协议、UI bridge 和更新协议分别从对应契约生成版本常量，不共用版本号。Go 与 JavaScript 的敏感文本脱敏通过 `scripts/testdata/redaction.json` 的共享向量校准。

动作调用前会检查 context 和事件终态。已发送动作在调用方停止等待后继续保留响应关联，终态等待宿主动作结算；超过 `ActionTimeout` 时不输出早于动作完成的终态，由宿主结束事件。已关闭事件不能继续调用 `event.Actions()` 发送动作。`adapter.send_unconfirmed` 和等待取消都不证明消息未发送，不能据此自动重试。

需要音视频处理的插件使用宿主环境变量：

- `RAYLEABOT_FFMPEG_PATH`
- `RAYLEABOT_FFPROBE_PATH`

插件应直接执行绝对路径，在变量缺失时报告媒体能力不可用，不重复打包 FFmpeg。

## raylea-plugin

统一工具位于 `sdk/go/cmd/raylea-plugin`：

```text
raylea-plugin inspect --plugin <plugin-root>
raylea-plugin inspect --artifact <expanded-artifact> [--target <platform>]
raylea-plugin pack --plugin <plugin-root> --binary <native-executable> --target <platform> --out <dist>
raylea-plugin build-go --plugin <plugin-root> [--backend <main-package>] --target <platform> --out <dist>
```

`pack` 和 `build-go` 的相对 `--binary`、`--out` 与 `--include source=destination` 源路径均以 `<plugin-root>` 解析；绝对路径保持原义。由此可从任意工作目录调用统一工具，而不会把产物写到调用者的当前目录。

- `inspect` 使用 `--plugin` 检查项目 manifest，或使用 `--artifact` 扫描展开产物并检查 manifest、目标平台和原生入口格式。
- `pack` 打包任意语言生成的当前目标平台原生可执行文件。
- `build-go` 从 `cmd/<plugin-id>` 构建 Go 后端，再调用统一 pack 流程。

Go 插件推荐结构：

```text
plugin-example/
  cmd/example/main.go
  internal/plugin/...
  ui/...
  templates/...
  go.mod
  info.json
```

每个插件直接使用统一构建器，无需维护 `tools/build` 包装器。构建器自动收集 `ui/`、模板、资源、许可证、第三方 notices 和 SPDX SBOM，并生成 artifact v2 及单根 ZIP。

正式目标平台：

- `windows-x64`
- `linux-x64`
- `macos-arm64`

## Vue UI SDK

`sdk/vue` 提供私有 workspace package `@rayleabot/plugin-ui`：

- `PluginUIBridgeClient` 完成 nonce-bound bridge v3 与 MessageChannel 握手。
- `usePluginHost` 暴露初始化状态、配置、secret configured-state 和 bridge 请求。
- `applyTheme` 把宿主主题 token 映射为插件 CSS variables。
- `contract.generated.ts` 从 bridge v3 schema 生成类型。

插件 UI 固定使用 Vue 3、TypeScript、Vite 和 `base: "./"`。所有页面共用 `management_ui.entry`，当前页面 ID 来自 `host.init.page.id`。页面不能读取管理 cookie、请求插件域 `/api` 或获取已保存 secret 明文。

## 本地联调

`plugin-workspace.local.json` 使用 workspace v2 连接本地插件仓库。插件 ID 从各仓库 `info.json` 推导；有 `go.mod` 的插件进入临时 go.work，无 Go module 的项目使用 `dist/native/<platform>/<plugin-id>[.exe]` 作为预构建原生入口。

开发 `watch` 会监听非 Go 插件的上述预构建入口；`dist` 下的其他生成产物仍被忽略，因此统一打包输出不会触发重复构建。

启动开发环境时，主仓库同步当前 Go/Vue SDK，并通过统一工具构建或打包后执行离线 `plugin dev-sync`。同步安装与商店安装共享 artifact 校验和原子替换边界。

## 验证

```bash
(cd sdk/go && go test ./...)
(cd sdk/vue && pnpm run typecheck && pnpm test && pnpm run build)
raylea-plugin build-go --plugin <plugin-root> --target linux-x64 --out dist
```

独立插件仓库自行运行后端测试、Vue 检查以及三个正式平台的 artifact 构建与校验。

## 相关文档

- [Plugin Manifest and Permissions](../permissions-and-manifest.md)
- [Plugin Protocol](../protocol.md)
- [Management UI](../management-ui.md)
- [Plugin Store and Independent Development](../store-and-development.md)

`EventContext.Bots` 保存全部已知适配器身份的隔离快照。`Bot` 对聊天事件按来源实例选择；多实例定时任务不推测身份，应从 `Bots` 明确选择实例并在主动发送时传入 `SourceAdapter`。
