# hello-python

这是一个与 `contracts/plugin-info.schema.json` 和 `contracts/plugin-protocol.schema.json` 对齐的最小 Python 示例插件。

用途：

- 展示最小 `info.json` 应如何声明
- 展示插件如何接收 `init`、返回 `init_ack`
- 展示插件如何接收最小 `event` 并返回 `result`

边界：

- 这是 contract-aligned example，不是生产插件模板
- 它只演示最小协议骨架，不展示完整 OneBot、子进程拉起、恢复或错误处理

常用 SDK helper 示例：

```python
plugin.message_history_get(request_id, "group", "123456", limit=20)
plugin.message_forward_get(request_id, forward_id="forward-001")
plugin.group_announcement_create(request_id, "123456", "维护窗口：今晚 23:00")
plugin.file_group_fs_list(request_id, "123456", folder_id="/reports")
plugin.napcat_group_sign_set(request_id, "123456")
```

```python
from rayleabot import ActionError, flash_file_segment, markdown_segment

segments = [
    markdown_segment("## 日报"),
    flash_file_segment({"name": "report.zip", "url": "https://example.com/report.zip"}),
]

try:
    plugin.http_request(request_id, "GET", "https://api.example.test/v1/data")
except ActionError as exc:
    plugin.logger_write(
        request_id,
        "warn",
        "request rejected",
        {"code": exc.code, "details": exc.details},
    )
```

若要真正调用这些 helper，需要在 manifest 的 `permissions.required` 或 `permissions.optional` 中声明对应 capability。
