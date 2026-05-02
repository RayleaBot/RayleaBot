# hello-node

这是一个与 `contracts/plugin-info.schema.json` 和 `contracts/plugin-protocol.schema.json` 对齐的最小 Node.js 示例插件。

用途：

- 展示最小 `info.json` 应如何声明
- 展示 Node.js SDK 的 `RayleaBotPlugin` 子类入口
- 展示 `PluginEventContext` 如何读取事件并返回 `result`

常用 SDK helper 示例：

```js
await ctx.messageHistoryGet('group', '123456', { limit: 20 })
await ctx.messageForwardGet({ forwardId: 'forward-001' })
await ctx.groupAnnouncementCreate('123456', '维护窗口：今晚 23:00')
await ctx.fileGroupFsList('123456', { folderId: '/reports' })
await ctx.napcatGroupSignSet('123456')
```

```js
import { ActionError, flashFileSegment, markdownSegment } from '@rayleabot/sdk'

const segments = [
  markdownSegment('## 日报'),
  flashFileSegment({ name: 'report.zip', url: 'https://example.com/report.zip' }),
]

try {
  await ctx.httpRequest('GET', 'https://api.example.test/v1/data')
} catch (error) {
  if (error instanceof ActionError) {
    await ctx.loggerWrite('warn', 'request rejected', {
      code: error.code,
      details: error.details,
    })
  }
}
```

若要真正调用这些 helper，需要在 manifest 的 `permissions.required` 或 `permissions.optional` 中声明对应 capability。
