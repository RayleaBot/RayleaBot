# RayleaBot Python SDK

`rayleabot-sdk` 是 Python 插件的开发安装入口，并固定依赖同版本的 `rayleabot-plugin-runtime`。

安装 SDK 后，插件代码统一导入运行时客户端：

```python
from rayleabot_runtime import RayleaBotPlugin, command
```

SDK 分发不提供 `rayleabot` 兼容导入。内置插件和 RayleaBot 发布包直接使用 `rayleabot-plugin-runtime`，无需安装 SDK 分发。
