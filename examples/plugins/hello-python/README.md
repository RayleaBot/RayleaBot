# hello-python

这是一个与 `contracts/plugin-info.schema.json` 和
`contracts/plugin-protocol.schema.json` 对齐的最小 Python 示例插件。

用途：

- 展示最小 `info.json` 应如何声明。
- 展示插件如何接收 `init`、返回 `init_ack`。
- 展示插件如何接收一个最小 `event` 并返回 `result`。

边界：

- 这是 contract-aligned example，不是生产插件模板。
- 它不展示 OneBot、插件子进程拉起、IPC、shutdown、错误恢复或 SDK 包装层。
- 入口文件只覆盖最小协议骨架，便于后续 AI / 人工实现对照。

常用 SDK helper 示例：

```python
plugin.message_history_get(request_id, "group", "123456", limit=20)
plugin.group_announcement_create(request_id, "123456", "维护窗口：今晚 23:00")
plugin.file_group_upload(request_id, "123456", "report.txt", "https://example.com/report.txt")
plugin.reaction_set(request_id, "msg_001", "👍")
plugin.luckylillia_friend_groups_get(request_id, "10001")
```

```python
from rayleabot import ActionError

try:
    plugin.http_request(request_id, "GET", "https://api.example.test/v1/data")
except ActionError as exc:
    retry_after = exc.details.get("retry_after_seconds")
    plugin.logger_write(
        request_id,
        "warn",
        "request rejected",
        {"code": exc.code, "retry_after_seconds": retry_after},
    )
```
