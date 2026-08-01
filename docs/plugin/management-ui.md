# Plugin Management UI

插件管理页是插件 artifact 中独立于 Go 后端的静态 Web 资源。正式契约以 `contracts/plugin-info.schema.json`、`contracts/plugin-management-ui.yaml`、`contracts/plugin-management-ui-bridge.schema.json` 与 `contracts/web-api.openapi.yaml` 为准。

## 构建与文件结构

- 管理页使用 Vue 3、TypeScript、Vite 和 `@rayleabot/plugin-ui`；需要复杂组件时按需引入 Ant Design Vue。
- Vite 固定 `base: "./"`。多页面内部路由只使用 hash routing，不能依赖插件域的服务端回退。
- 插件自己的 `pnpm build` 产出 `ui/index.html` 与哈希资源；`pluginbuild.Build` 把这些文件写入同一平台 artifact。
- Vue 资源不嵌入 Go 二进制，服务端按 `artifact.json` 中的 `ui` 文件集合独立校验和读取。
- contract 驱动的消息渲染模板继续位于 `templates/`，不使用 Vue。

```json
{
  "management_ui": {
    "pages": [
      {
        "id": "config",
        "label": "配置页面",
        "entry": "ui/index.html"
      }
    ]
  }
}
```

`pages[].id` 是稳定页签标识，`label` 是宿主标题，`entry` 必须是 artifact 内已登记为 UI 文件的相对 HTML 路径。

## 独立插件域

- 插件 UI 与管理面使用不同 origin。宿主 iframe 只加载当前插件 artifact 的 `ui/` 文件。
- 本机模式默认派生 `http://p-<sha256(plugin_id)[0:16]>.plugins.localhost:<port>`。
- LAN 或反向代理模式必须显式配置 `web.plugin_ui_origin_template`，并保留 `{plugin_host}` 占位符；解析结果不能与管理面 origin 相同。
- 管理 cookie 是 admin origin 的 host-only cookie，插件 origin 不接收 cookie。
- 插件域没有 `/api` 路由，不开放管理端 CORS，不提供目录枚举，也不允许路径越界。
- 响应应用严格 CSP：脚本、样式、图片和字体只允许 artifact 自身所需来源，`connect-src 'none'`，`frame-ancestors` 只允许管理面 origin。

## Bridge v2

首次加载只允许两条 window 消息：

1. iframe 发送带一次性 nonce 的 `page.ready`。
2. 宿主同时校验 `event.source`、精确 origin 和 nonce，创建 `MessageChannel`，通过 `host.connect` 转交一个端口。

端口转交后，所有 `host.init`、设置、密钥、调度器、模板、协议目标和插件动作消息都只通过绑定端口传输。重复 nonce、错误窗口、错误 origin、bridge v1 或继续使用 window 消息都不会获得宿主能力。

`host.init` 只包含：

- 插件 ID、名称、版本和当前页面；
- 当前设置与 secret 是否已配置的布尔映射；
- 主题、语言和允许能力。

`ui.resize` 可请求内容高度，宿主最终限制在 320–1600px。

## 设置与密钥

插件 UI 不能直接调用管理 API。宿主在校验 bridge 消息后代表当前插件调用受保护接口：

| Bridge | HTTP | 语义 |
| --- | --- | --- |
| `settings.reload` | `GET /api/plugins/{plugin_id}/settings` | 读取当前设置 |
| `settings.save` | `PUT /api/plugins/{plugin_id}/settings` | 覆盖非敏感设置 |
| `secrets.status.reload` | `GET /api/plugins/{plugin_id}/secrets` | 只读取是否已配置 |
| `secrets.set` | `PUT /api/plugins/{plugin_id}/secrets` | 覆盖选定密钥，不回显明文 |
| `secrets.delete` | `DELETE /api/plugins/{plugin_id}/secrets` | 显式删除选定密钥 |

已保存的密钥明文不会出现在 GET 响应、`host.init`、后续 bridge 消息或网络回包中。运行中的 Go 插件仍可在声明 `secret.read` 后通过 local action 读取自身命名空间中的单个值。

## 其他受控能力

- `scheduler.trigger` 请求宿主触发当前插件的任务。
- `render_template.open` 请求宿主跳转到正式模板工作区。
- `protocol.targets.reload` 与 `protocol.identities.resolve` 请求宿主读取受保护的 OneBot 目标信息。
- `plugin.action.invoke` 把页面动作发送给所属 Go 插件；宿主不代替插件执行业务逻辑。
- `trust.level = unverified` 的来源在首次打开、版本变化或来源变化后需要重新确认。

插件 UI 永远不获得管理 token、Pinia store、通用管理 API、任意跨插件数据或跨 origin 网络访问能力。

## 相关文档

- [Capabilities and Manifest](./capabilities-and-manifest.md)
- [SDK](./sdk/README.md)
- [Management Surface](../user/management-surface.md)
