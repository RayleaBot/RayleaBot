# Plugin Management UI

本页说明插件内置管理页的正式能力、文件结构和与 Web 管理面的交互边界。

正式契约以 `contracts/plugin-info.schema.json`、`contracts/plugin-management-ui.yaml`、`contracts/plugin-management-ui-bridge.schema.json` 与 `contracts/web-api.openapi.yaml` 为准。

## 能力边界

- 每个插件可声明一个内置管理页入口。
- 管理页作为插件详情页内的工作区显示，路径保持在 `/plugins/:id`。
- 管理页资源来自插件包内静态文件。
- 管理页只允许读取和保存插件自己的设置与敏感值。
- 管理页不直接获得管理 Token、全局 store 或通用管理 API 调用能力。

## Manifest 字段

```json
{
  "management_ui": {
    "entry": "web/index.html",
    "label": "配置页面"
  }
}
```

- `entry`：插件包内相对路径，指向 HTML 入口文件。
- `label`：插件详情页工作区标题；省略时使用默认标题。

## 静态资源

- 正式公共路由前缀：`/plugin-ui/{plugin_id}/...`
- 宿主只读取 `management_ui.entry` 所在目录下的资源文件。
- 不提供目录枚举。
- 越界路径、缺失文件、无管理页入口的插件和无效插件都会被拦截。

## 设置读取与保存

| 接口 | 作用 |
| --- | --- |
| `GET /api/plugins/{plugin_id}/settings` | 读取当前生效设置 |
| `PUT /api/plugins/{plugin_id}/settings` | 保存插件设置 |

- `GET` 返回 `default_config` 叠加已持久化值后的当前设置。
- `PUT` 请求体固定为 `values: object`。
- `PUT` 响应返回 `changed_keys` 与最新 `values`。
- 设置保存后，已运行插件继续通过现有 `config.changed` 事件链接收配置变化。

## 敏感值读取与保存

| 接口 | 作用 |
| --- | --- |
| `GET /api/plugins/{plugin_id}/secrets` | 读取插件自己的敏感值 |
| `PUT /api/plugins/{plugin_id}/secrets` | 保存或删除插件敏感值 |

- `GET` 返回插件 secret 命名空间内的明文值，只面向已登录的受保护管理面。
- `PUT` 请求体固定为 `values: object<string, string>`，可带 `deleted_keys: string[]`。
- 插件 runtime 通过 `secret.read` 读取自身命名空间内的单个敏感值。

## 桥接消息

管理页 iframe 与宿主页只使用正式 `postMessage` 消息：

- `page.ready`
- `host.init`
- `settings.reload`
- `settings.save`
- `settings.changed`
- `secrets.reload`
- `secrets.save`
- `secrets.changed`
- `error`

`host.init` 会提供：

- `plugin_id`
- 当前插件展示信息最小集
- `trust`
- `default_config`
- 当前 `settings`
- 当前 `secrets`
- 页面标题

## 未验证来源确认

- `trust.level = unverified` 的插件首次打开管理页时，插件详情页先显示确认卡。
- 确认记录按 `plugin_id + version + package_source_type + package_source_ref` 保存。
- 插件版本或来源变化后需要重新确认。

## 适用场景

- 插件自己的表单化设置
- 插件自己的 token、API key、webhook secret 等敏感值设置
- 插件自己的轻量状态说明
- 与 `default_config` 对应的管理项展示

## 不包含的能力

- 多入口插件管理站点
- 远端页面接入
- 自动表单生成
- 全局配置编辑
- 任意管理 API 代理
- Launcher 内嵌同类页面

## 相关文档

- [Capabilities and Manifest](./capabilities-and-manifest.md)
- [Management Surface](../user/management-surface.md)
