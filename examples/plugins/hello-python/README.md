# hello-python

这是一个与 `contracts/plugin-info.schema.json` 和 `contracts/plugin-protocol.schema.json` 对齐的最小 Python 示例插件。

用途：

- 展示最小 `info.json` 应如何声明
- 展示 Python SDK 的 `RayleaBotPlugin` 子类入口
- 展示 `EventContext` 如何读取事件并返回 `result`

常用 SDK helper 示例：

```python
ctx.message_history_get("group", "123456", limit=20)
ctx.message_forward_get(forward_id="forward-001")
ctx.group_announcement_create("123456", "维护窗口：今晚 23:00")
ctx.file_group_fs_list("123456", folder_id="/reports")
ctx.napcat_group_sign_set("123456")
```

```python
from rayleabot import ActionError, flash_file_segment, markdown_segment

segments = [
    markdown_segment("## 日报"),
    flash_file_segment({"name": "report.zip", "url": "https://example.com/report.zip"}),
]

try:
    ctx.http_request("GET", "https://api.example.test/v1/data")
except ActionError as exc:
    ctx.logger_write(
        "warn",
        "request rejected",
        {"code": exc.code, "details": exc.details},
    )
```

若要真正调用这些 helper，需要在 manifest 的 `capabilities` 中声明对应 capability；`http.request`、`storage.file` 和 `event.expose_webhook` 还需要在 `capability_parameters` 中声明边界参数。
