# 人工 Smoke 登记

这些用例需要外部平台凭据或人工操作，不属于默认 `go test ./...` 的自动覆盖。执行结果应记录时间、源码提交、用例、平台与观察结果；没有执行时填写“未执行”，不能将默认测试通过视为 live 验收。

## 公开发行物

从公开 Release 的 Assets 和公开插件目录开始，不复用源码构建、开发插件镜像或已有运行数据。记录完整版本标签、提交 SHA、nightly 运行 URL、下载页、目录版本、平台、协议端版本、执行时间与观察结果。

| 用例 | 前置条件与操作 | 通过条件 |
| --- | --- | --- |
| Windows 完整包、Linux server 全流程 | 在干净目录下载并安装，初始化管理员，连接 NapCat OneBot11，发送真实命令；从公开商店安装兼容插件，使用第二个公开插件版本执行更新，再备份并恢复到新的空目录 | 安装、初始化、命令回复、商店安装、插件更新和恢复均成功，恢复后的配置与数据一致 |
| macOS arm64 实验性安装 | 在 Apple Silicon 实机从公开 Release 下载，按部署说明启动并初始化 | Launcher 与 Server 可启动、管理员可登录；此结果不代表完整平台验收 |
| 下载产物的恢复演练 | 对下载包内的 Server 执行 `go run ./tools/cmd/rehearse-current-recovery --server <下载的Server路径> --output <尚不存在的结果目录>` | 脚本成功，结果目录保存进程日志与演练结果 |

首个 v4 插件只有一个公开版本时，插件更新填写“未执行”；待兼容补丁版公开后补验。公开目录没有兼容包、缺少真实协议凭据或缺少平台实机时，同样保留“未执行”，不能用本地 fixture 代替。

`0.4.0-beta.1` 低于 manifest v4 的最低核心版本要求 `0.4.0`，不能用于插件安装、更新与真实插件命令验收。该预发布先执行核心安装与初始化，其余项目等待满足版本要求的公开产物。

| 时间 | 标签 / 提交 | 平台与用例 | nightly / 下载页 / 目录版本 | 结果与观察 |
| --- | --- | --- | --- | --- |
| — | v0.4.0 首次公开候选 | 公开发行物全流程 | 待实际发布与选择验证环境 | 未执行 |

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
