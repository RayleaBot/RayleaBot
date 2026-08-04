# Plugin SDK

RayleaBot 插件开发由 Go 后端 SDK、artifact 构建器和 Vue 管理页 SDK 组成。插件运行期只读取已编译产物，不使用源码 SDK、语言解释器或依赖安装器。

## Go SDK

`sdk/go` 是独立 Go module，公开入口为：

```go
err := rayleabot.Run(ctx, rayleabot.Options{
    PluginID:              "example.plugin",
    Subscriptions:         []string{"message.*"},
    MaxConcurrentHandlers: 4,
}, rayleabot.HandlerFunc(func(ctx context.Context, event *rayleabot.EventContext) error {
    return event.SendText("ok")
}))
```

`EventContext` 提供当前事件、request ID、插件 ID、bot 身份、允许能力、超级管理员与命令前缀。每个事件只能发送一次终态 `Result`、`Fail`、`Send`、`SendText` 或 `Reply`；重复终态调用会返回错误。

`event.Actions()` 提供 request-bound typed helpers：

- 非终态消息、日志、KV、文件、HTTP、配置、插件列表与 secret；
- 治理黑白名单、命令策略、scheduler、webhook、渲染与三方账号；
- 已冻结的 OneBot 单动作和 provider 扩展动作；
- `Call` 作为正式 action 名称已经进入 contract 时的通用入口。

SDK 为每个 local action 分配独立 request ID，并通过父事件 request ID 关联并发响应。stdout 只写 JSONL，使用串行 writer；日志写 stderr。运行时处理 `init/init_ack`、`ping/pong`、shutdown、超时、并发上限和 panic 隔离，panic 只终止当前事件并返回受控错误。

## Artifact 构建器

每个插件拥有独立 `go.mod`、`info.json` 和薄 `tools/build` 入口：

```go
package main

import "github.com/RayleaBot/RayleaBot/sdk/go/pluginbuild/buildcmd"

func main() { buildcmd.Main("templates") }
```

也可以直接调用 `pluginbuild.Build(ctx, Config)`。构建器执行：

1. 校验 manifest v2 与目标平台；
2. 使用 `CGO_ENABLED=0`、`-trimpath`、`-buildvcs=false` 和无 build ID 构建 Go 后端；
3. 若存在 `ui/package.json`，执行插件自己的 `pnpm build`；
4. 收集 UI、模板、数据、`LICENSE`、第三方 notices 与 SPDX SBOM；
5. 生成 `artifact.json`，并输出确定性单根目录 ZIP 与可选展开目录。

```text
example.plugin/
  info.json
  artifact.json
  bin/example[.exe]
  ui/index.html
  ui/assets/...
  templates/...
  LICENSE
  THIRD_PARTY_NOTICES.md
  sbom.spdx.json
```

正式目标为 `windows-x64`、`linux-x64` 和 `macos-arm64`。`artifact.json` 枚举除自身外的每个文件，记录角色、大小和 SHA-256。ZIP 中 Unix 后端固定为 `0755`。

## Vue UI SDK

`sdk/vue` 提供私有 workspace package `@rayleabot/plugin-ui`：

- `PluginUIBridgeClient` 完成 nonce-bound bridge v2 与 `MessageChannel` 握手；
- `usePluginHost` 暴露初始化状态、设置、secret configured-state 和 bridge 请求；
- `applyTheme` 把宿主主题 token 投影为插件 CSS variables；
- `contract.generated.ts` 提供 bridge v2 类型。

插件 UI 固定使用 Vue 3、TypeScript、Vite 与 `base: "./"`。页面只能通过 SDK 绑定的端口请求宿主能力，不能请求插件域 `/api`，也不能读取管理 cookie 或已保存密钥明文。

## 验证

```bash
(cd sdk/go && go test ./...)
(cd sdk/vue && pnpm run typecheck && pnpm test && pnpm build)
(cd ../RayleaBotPlugins/plugin-fortune && go run ./tools/build -target linux-x64 -out dist)
```

主仓库 CI 验证 Go/Vue SDK、示例、安装器和测试 fixture。每个独立插件仓库自行执行 `go test -race ./...`、Vue typecheck/Vitest/build，并为三个正式平台构建和校验 artifact。

## 许可证与发布边界

核心 SDK 和示例使用主仓库 `AGPL-3.0-only` 许可证；独立插件按各自仓库的 `LICENSE` 发布。RayleaBot 应用包不包含业务插件 artifact，也不包含插件 `.go`、`.ts`、`.vue`、测试、源码 SDK、`node_modules` 或语言运行时。

## 相关文档

- [Capabilities and Manifest](../capabilities-and-manifest.md)
- [Protocol](../protocol.md)
- [Management UI](../management-ui.md)
- [Plugin Store and Independent Development](../store-and-development.md)
