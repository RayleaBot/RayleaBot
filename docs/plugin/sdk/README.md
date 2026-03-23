# Plugin SDK Docs

本目录用于说明 RayleaBot 官方 Python / Node.js SDK 的当前使用边界。

## 当前 SDK 角色

- SDK 为当前插件协议提供便利封装，服务于已落地的 runtime 主链路和示例插件。
- 文档范围以 `init` / `init_progress` / `init_ack`、事件接收、`ping/pong`、`shutdown` 与当前正式消息动作 surface 为主。
- SDK 应与 builtin 资源、示例插件和当前 dispatcher / scheduler 投递模型保持一致。

## 当前适用范围

- Python / Node.js SDK 只覆盖当前正式协议与已落地 action。
- Python SDK 当前提供 `send_message`、`send_reply`，并补充 `logger_write`、`storage_get`、`storage_set`、`storage_delete`、`storage_list`、`storage_file_read`、`storage_file_write`、`storage_file_delete`、`storage_file_list`、`http_request`。
- Node.js SDK 当前提供 `sendMessage`、`sendReply`，并补充 `loggerWrite`、`storageGet`、`storageSet`、`storageDelete`、`storageList`、`storageFileRead`、`storageFileWrite`、`storageFileDelete`、`storageFileList`、`httpRequest`。
- 两套 SDK 的本地 action helper 默认使用 30 秒超时，并在当前事件处理期间等待同 `request_id` 的 `result` / `error` 响应。
- 更宽的调试流、复杂流式回传、批量消息和额外 action 仍未进入正式协议范围。
- SDK 说明需要与 `contracts/plugin-protocol.schema.json`、`docs/plugin/` 和 `examples/plugins/` 保持一致。

## Node.js Rich Reply 示例

```js
plugin.sendReply(requestId, event.event_id, [
  { type: 'text', data: { text: '已收到，开始处理。' } },
], { fallbackToSendIfMissing: true });
```

## Python Local Action 示例

```python
response = plugin.http_request(request_id, "GET", "https://example.com/")
content = response.get("body_text")
if content is not None:
    plugin.storage_file_write(request_id, "cache/example.html", content_text=content)
else:
    plugin.storage_file_write(request_id, "cache/example.bin", content_base64=response["body_base64"])
plugin.logger_write(request_id, "info", "content cached", {"status_code": response.get("status_code")})
```

## 维护规则

- SDK 说明必须服从正式插件协议契约。
- 若 SDK 需要新增字段、消息类型或 action，先更新 `contracts/`，再补 fixtures、示例和 SDK 文档。
- SDK 是协议的实现便利层，不单独裁决对外语义。
