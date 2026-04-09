# hello-node

这是一个与 `contracts/plugin-info.schema.json` 和
`contracts/plugin-protocol.schema.json` 对齐的最小 Node.js 示例插件。

用途：

- 展示最小 `info.json` 应如何声明。
- 展示插件如何接收 `init`、返回 `init_ack`。
- 展示插件如何接收一个最小 `event` 并返回 `result`。

边界：

- 这是 contract-aligned example，不是生产插件模板。
- 它不展示 OneBot、插件子进程拉起、IPC、shutdown、错误恢复或 SDK 包装层。
- 入口文件只覆盖最小协议骨架，便于后续 AI / 人工实现对照。

常用 SDK helper 示例：

```js
await plugin.messageHistoryGet(requestId, 'group', '123456', { limit: 20 })
await plugin.groupAnnouncementCreate(requestId, '123456', '维护窗口：今晚 23:00')
await plugin.fileGroupUpload(requestId, '123456', 'report.txt', 'https://example.com/report.txt')
await plugin.reactionSet(requestId, 'msg_001', '👍')
await plugin.napcatMessageEmojiLikeSet(requestId, 'msg_001', '128512')
```
