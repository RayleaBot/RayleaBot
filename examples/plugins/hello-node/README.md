# hello-node

这是一个与 `contracts/plugin-info.schema.json` 和 `contracts/plugin-protocol.schema.json` 对齐的最小 Node.js 示例插件。

用途：

- 展示最小 `info.json` 应如何声明
- 展示插件如何接收 `init`、返回 `init_ack`
- 展示插件如何接收最小 `event` 并返回 `result`

边界：

- 这是 contract-aligned example，不是生产插件模板
- 它只演示最小协议骨架，不展示完整 OneBot、子进程拉起、恢复或错误处理

常用 SDK helper 示例：

```js
await plugin.messageHistoryGet(requestId, 'group', '123456', { limit: 20 })
await plugin.messageForwardGet(requestId, { forwardId: 'forward-001' })
await plugin.groupAnnouncementCreate(requestId, '123456', '维护窗口：今晚 23:00')
await plugin.fileGroupFsList(requestId, '123456', { folderId: '/reports' })
await plugin.napcatGroupSignSet(requestId, '123456')
```

```js
import { ActionError, flashFileSegment, markdownSegment } from '@rayleabot/sdk'

const segments = [
  markdownSegment('## 日报'),
  flashFileSegment({ name: 'report.zip', url: 'https://example.com/report.zip' }),
]

try {
  await plugin.httpRequest(requestId, 'GET', 'https://api.example.test/v1/data')
} catch (error) {
  if (error instanceof ActionError) {
    await plugin.loggerWrite(requestId, 'warn', 'request rejected', {
      code: error.code,
      details: error.details,
    })
  }
}
```

若要真正调用这些 helper，需要在 manifest 的 `permissions.required` 或 `permissions.optional` 中声明对应 capability。
