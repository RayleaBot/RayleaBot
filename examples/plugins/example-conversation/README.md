# 三轮会话示例

在聊天中发送当前前缀加“会话演示”，随后依次回复角色、操作和“确认”。错误输入会重新询问，仍使用同一对话 ID。业务草稿保存在回调闭包中，最终确认前不写 KV；确认后按完整聊天身份保存五分钟。

示例使用单并发 SDK。等待期间已结束原事件，后续回复使用新的 EventContext。发起或处理期间的额外消息走普通流程，不会缓冲为下一轮答复；超时通知尽力投递。

从仓库根目录构建当前平台 artifact：

```powershell
go run ./sdk/go/cmd/raylea-plugin build-go --plugin examples/plugins/example-conversation --target windows-x64 --out ../../../dist/example-conversation
```

Linux 和 macOS 使用对应的 linux-x64、macos-arm64。SDK 依赖沿用仓库内示例的本地 module 引用，用于源码开发验证，不代表已发布 SDK 标签的消费者验证。
