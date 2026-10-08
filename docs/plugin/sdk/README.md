# Plugin SDK

RayleaBot 插件运行时与实现语言无关。主仓库提供 Go 后端 SDK、通用 artifact 工具和 Vue 管理页 SDK；插件运行时只读取已编译产物。

`sdk/go`（含 `raylea-plugin` 工具）与 `sdk/vue` 使用 MIT 许可，主仓库其余部分使用 AGPL-3.0。插件只链接 SDK 时不受主仓库 AGPL 约束，可以自行选择许可。

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

插件 ID、并发度、配置、管理员和命令前缀都来自 init。`EventContext` 提供：

- 当前事件与 request ID。
- 宿主分配的插件 ID。
- 当前 Bot 身份。
- 隔离的完整配置快照。
- 超级管理员和命令前缀。
- 宿主当前生效的 `Location`；显示时间使用 `timestamp.In(event.Location)`，动作边界可通过 `event.Actions().TimeLocation()` 取得同一个时区。
- 当前期限 `event.Deadline()`。handler 的 context 在事件帧的 `deadline_at_ms` 结束，转入后台后改按后台期限；handler 应在 context 结束后尽快返回，到期的事件不再发送默认终态。

每个事件只能发送一次 `Result`、`Fail`、`Send`、`SendText` 或 `Reply` 终态。`Reply` 仍使用 protocol v4 的统一 `message.send` action。

消息处理可用 `event.ResultWithPropagation(nil, rayleabot.PropagationStop)` 停止后续优先级，或用 `PropagationContinue` 覆盖 manifest 的 block。同层已经执行的动作不会撤销；非消息事件不能指定传播结果。

计划任务触发用 `event.Event.TaskID()` 读取 `scheduler.create` 的任务 ID 并按它分派；遇到不认识的任务 ID 时调用 `event.Actions().SchedulerDelete` 删除该任务。

`event.Actions()` 提供 request-bound typed helpers：

- 非终态消息、日志、KV、配置写入、插件列表和 secret。
- `CallService(ctx, ServiceCallRequest, output)` 跨插件服务调用。
- 治理、scheduler、渲染和浏览器会话动作。
- OneBot 单动作与 provider 扩展动作。
- 已进入正式 contract 的通用 `Call`。

临时 KV 使用 `event.Actions().KVSetWithOptions(ctx, "draft", value, rayleabot.KVSetOptions{TTL: 5 * time.Minute})`，返回可选的 `ExpiresAtMS`。零 TTL 和既有 `KVSet` 都表示永久写入并清除旧期限；负数、正的非整秒或超过 31536000 秒的 TTL 会在发送前报错。显式 nil 值作为 JSON null 写入。需要进程内条件写入时由插件自行同步。

OneBot 和 provider typed helpers 默认由宿主按当前聊天事件选择实例。定时任务等平台事件需要主动指定实例时，使用 `event.Actions().ForOneBotAdapter("second-bot").GroupInfoGet(ctx, groupID)`；返回的动作视图不会改变其他调用的实例。显式实例仍须与聊天父事件一致，多实例且没有选择信息时宿主拒绝调用。

SDK 串行写 stdout JSONL，日志写 stderr；负责 request 关联、并发、ping/pong、关闭、panic 隔离和配置快照原子替换。

Server 与 Go SDK 的 wire 模型由 `tools/cmd/generate-plugin-wire` 从正式协议 schema 生成，分别放在 Server 的 `internal/plugins/pluginwire` 与 SDK 的 `internal/pluginwire` 中。SDK 可用 `GOWORK=off go test ./...` 独立验证，不依赖 Server internal 包。修改契约后运行该生成器和 `node scripts/generate-runtime-schemas.mjs`；两者的 `--verify` 检查缺失、变化和多余的自有生成产物。

生成器只负责传输结构投影：required 字段保留零值，语义需要区分缺省的字段使用指针或 RawMessage，KV 的显式 `null` 不等同于缺少 value。动作 data、动态配置和扩展 payload 保留 JSON 边界。Server 只对 `plugin dev-sync` 安装的开发插件按内嵌 schema 校验入站帧；SDK 与正式插件不逐帧校验，双方仍检查帧字节上限、请求关联和生命周期状态。

artifact、manifest 与运行时协议分别从对应契约生成版本常量，不共用版本号。Go 与 JavaScript 的敏感文本脱敏通过 `scripts/testdata/redaction.json` 的共享向量校准。

动作调用前会检查 context 和事件终态。已发送动作在调用方停止等待后继续保留响应关联，终态等待宿主动作结算；超过 `ActionTimeout` 时不输出早于动作完成的终态，由宿主结束事件。已关闭事件不能继续调用 `event.Actions()` 发送动作。`adapter.send_unconfirmed` 和等待取消都不证明消息未发送，不能据此自动重试。

插件持久化文件写入宿主传入的 `RAYLEABOT_PLUGIN_DATA_DIR`。该绝对路径指向 `data/plugins/<plugin_id>/`，进程启动前已创建，随备份与恢复保留；插件直接读写，不经过宿主动作。

可重建缓存、下载与媒体中间文件写入 `RAYLEABOT_PLUGIN_CACHE_DIR`。该绝对路径指向运行根目录的 `cache/plugins/<plugin_id>/`，进程启动前已创建，不随业务数据备份；插件负责清理。

宿主同时把 `TMP`、`TEMP`、`TMPDIR` 设置为该缓存目录内的 `tmp/`，插件及其子进程调用系统临时目录 API 时使用此目录。

需要音视频处理的插件使用宿主环境变量：

- `RAYLEABOT_FFMPEG_PATH`
- `RAYLEABOT_FFPROBE_PATH`

插件应直接执行绝对路径，在变量缺失时报告媒体能力不可用，不重复打包 FFmpeg。

### 后台事件

分钟级流程在同一个 handler 中按顺序完成，不需要切片续跑：

```go
if _, err := event.Detach(ctx, nil); err != nil {
    return err // 宿主拒绝时事件仍在前台
}
// ctx 改在后台期限结束，event.Actions() 继续可用。
if err := refreshAll(ctx, event.Actions()); err != nil {
    return err
}
return event.SendText("更新完成")
```

- `Detach(ctx, result)` 发送 `event.detach` 并返回后台期限。宿主以 `result` 完成投递：管理动作把它返回管理页，消息继续分层，计划任务记为已投递；`result` 必须编码为 JSON 对象，nil 表示空结果。消息事件用 `DetachWithPropagation(ctx, result, rayleabot.PropagationStop)` 同时决定后续优先级。
- 可转入的事件为私聊与群消息、计划任务触发和管理动作，每个事件一次。宿主拒绝时返回 `*ActionError`：不可转入或重复转入为 `platform.invalid_request`，后台事件达到插件上限为可重试的 `platform.rate_limited`。
- 转入后事件交还并发许可，同一进程的其他事件不再等待它；`event.Deadline()` 返回后台期限，`event.Detached()` 报告已转入。转入前从 handler context 派生、时限长于事件期限的子 context 会随转入一并延长，需要独立时限时在转入后派生。
- 转入后 `SendText`、`Send`、`Reply` 先发送普通消息动作，再以 `Result` 结束事件；`Result` 与 `Fail` 照常结束事件；`ResultWithPropagation` 返回错误，因为传播已在转入时决定。`Ask` 与 `SessionWait` 在后台事件中不可用。
- 后台期限到达时 context 结束，宿主按 `plugin.event_timeout` 结束事件；插件停止或重载时后台事件被取消。已开始的外部副作用由插件自行保证幂等，宿主不重放。

### 回调式会话

`event.Ask(ctx, prompt, options, next)` 登记等待、以非终态动作发送提示，并成功结束当前事件；返回后应结束当前 Handler。next 是 `HandlerFunc`，接收下一条消息的新 EventContext。回调中再次 Ask 会复用当前对话 ID；scope 只用于新建，业务步骤保存在闭包中。

`SessionWaitOptions` 提供 Scope、整秒 Timeout 和可选的 NotifyOnExpire。零 Timeout 使用宿主默认值；负数、非整秒或超过 600 秒会报错。等待期间不占旧请求的执行许可；本地超时回收回调，不使用已结束的 Context 发动作。提示发送失败时撤销登记并返回错误，调用方应将错误返回给 Handler 收尾。

可选超时通知进入普通 Handler 的 `session.expired` 分支，通过 `event.Event.Session` 读取引用；可以使用该新 Context 发送超时提示。回调本身只接收消息回复。通知丢失不会保留等待回调，已失效回调的迟到消息也不会转交普通业务处理器。

需要自行处理会话事件时使用 `event.Actions().SessionWait`，成功结束当前事件后在 Handler 读取 `event.Event.Session`。再次等待传 SessionID；新建不传。`SessionFinish` 可在同一插件进程的任意活动事件中结束等待。回调缓存只是本地续接映射，宿主仍持有正式路由与期限。

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

每个插件直接使用统一构建器，无需维护 `tools/build` 包装器。构建器自动收集 `ui/`、模板、资源、许可证和第三方 notices，并生成 artifact v2 及单根 ZIP。

正式目标平台：

- `windows-x64`
- `linux-x64`
- `macos-arm64`

## 插件服务 SDK

提供者在 manifest 声明 `services`，在 `Options.Services` 中注册相同的名称、版本和方法。`ServiceHandler` 接收 `context.Context`、`EventContext` 和 `ServiceRequest`，返回业务对象或 error；服务专用插件可让普通 Handler 为 nil。

调用者使用 `event.Actions().CallService(ctx, request, &output)`，显式提供目标插件、服务、版本、方法和对象参数。业务参数中的空对象、空数组与 null 保持区别；参数和无类型业务结果中的数字使用 `json.Number`，也可解码到调用者定义的结果 struct。

提供者返回 `ActionError` 可公开指定 code、message、details；普通 Go error 只返回通用失败，默认日志不输出可能包含业务正文的错误字符串。提供者须响应 context 取消；已发生的外部副作用由自身业务处理幂等与核对。

[提供者与调用者示例](../../../examples/plugins/example-service-provider/README.md)可独立构建并通过真实 JSONL 进程联调。

## Vue UI SDK

`sdk/vue` 提供私有 workspace package `@rayleabot/plugin-ui`：

- `PluginUIClient` 从 `/plugin-ui/{plugin_id}/` 路径识别插件、从 `page` 参数读取页面 ID，并以当前会话直接调用插件范围的管理 API；写操作沿用管理面的 CSRF 头。
- `usePluginHost` 暴露初始化状态、配置、secret configured-state 和客户端。
- `invokeAction` 等待管理动作结果；`apiRequest` 调用其他管理接口，失败时抛出带稳定 `code` 的 `PluginUIError`。
- `applyTheme` 把宿主主题变量映射为插件 CSS variables，`usePluginHost` 在宿主切换主题时同步更新。

官方插件 UI 与 `@rayleabot/plugin-ui` 使用 Vue 3、TypeScript、Vite 和 `base: "./"`。所有页面共用 `management_ui.entry`，页面不能获取已保存 secret 明文。

`@rayleabot/plugin-ui` 不发布到 npm。独立插件仓库在 `.rayleabot-sdk-ref` 记录主仓库的提交或 SDK 标签，CI 检出该版本，把 `sdk/vue` 复制到 `.rayleabot/sdk/vue`，UI 的 `package.json` 以 `link:../.rayleabot/sdk/vue` 引用；Go 后端的 `go.mod` 使用同一版本的 `sdk/go`。UI 的 `tsconfig.json` 把 `vue` 映射到 UI 自身的 `node_modules/vue`，Vite 配置 `resolve.dedupe: ["vue"]`，这样不必安装 SDK 目录的依赖，构建产物也只含一份 Vue。官方插件的发布工作流即按此方式构建。

## 本地联调

`plugin-workspace.local.json`（workspace v2）连接本地插件仓库，插件 ID 从各仓库 `info.json` 推导；非 Go 项目以 `dist/native/<platform>/<plugin-id>[.exe]` 作为预构建原生入口。工作区模式、同步方式与监听规则见[插件商店与独立开发](../store-and-development.md)。

## 验证

```bash
(cd sdk/go && go test ./...)
(cd sdk/vue && pnpm run typecheck && pnpm test && pnpm run build)
raylea-plugin build-go --plugin <plugin-root> --target linux-x64 --out dist
```

独立插件仓库自行运行后端测试、Vue 检查以及三个正式平台的 artifact 构建与校验。

## 相关文档

- [Plugin Manifest](../manifest.md)
- [Plugin Protocol](../protocol.md)
- [Management UI](../management-ui.md)
- [Plugin Store and Independent Development](../store-and-development.md)
