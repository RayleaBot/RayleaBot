# 人工 Smoke 登记

这些用例需要外部平台凭据或人工操作，不属于默认 `go test ./...` 的自动覆盖。执行结果应记录时间、源码提交、用例、平台与观察结果；没有执行时填写“未执行”，不能将默认测试通过视为 live 验收。

## QQ 官方机器人

两项用例使用 `manual_smoke` build tag。凭据只通过当前终端的 `QQ_APP_ID`、`QQ_APP_SECRET` 环境变量提供；显式选择用例后，缺少必需输入会失败。

| 用例 | 前置条件与操作 | 通过条件 |
| --- | --- | --- |
| [网关握手](../../server/internal/bot/adapters/qqofficial/live_gateway_test.go) | 使用已开通所需 intents 的测试机器人；允许连接 QQ 开放平台 | 在超时前收到 READY 和 bot identity，停止客户端成功 |
| [媒体回复](../../server/internal/bot/adapters/qqofficial/live_media_test.go) | 另设置 `QQ_LIVE_IMAGE` 为本机可读图片；启动后向测试机器人发送一条消息。用例会回复文本和图片 | 上传及回复成功，平台返回非空 message ID，客户端停止成功；人工确认会话中图片可见 |

从仓库根目录分别运行：

```powershell
go -C server test -tags manual_smoke ./internal/bot/adapters/qqofficial -run '^TestLiveGatewayHandshake$' -count=1 -v -timeout 1m
go -C server test -tags manual_smoke ./internal/bot/adapters/qqofficial -run '^TestLiveMediaReply$' -count=1 -v -timeout 5m
```

媒体回复的人工可见结果与 API 确认分别记录；只通过握手不代表媒体能力已经验收。自动测试中的本地 HTTP/WebSocket fixtures 继续负责协议解析、鉴权错误、重连、限额和失败处理。
