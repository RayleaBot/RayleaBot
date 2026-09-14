# Plugin Management UI

插件管理页是插件 artifact 中独立于原生后端的静态 Web 资源。正式契约以 `contracts/plugin-info.schema.json`、`contracts/plugin-management-ui.yaml` 与 `contracts/web-api.openapi.yaml` 为准。

## 构建与文件结构

- 官方管理页使用 Vue 3、TypeScript、Vite 和 `@rayleabot/plugin-ui`；界面组件及样式由插件自行打包。第三方页面可采用其他静态 Web 技术，但仍需满足管理页契约。
- Vite 固定 `base: "./"`。多页面内部路由只使用 hash routing，不能依赖服务端回退。
- 插件自己的 `pnpm build` 产出 `ui/index.html` 与哈希资源；`pluginbuild.Build` 把这些文件写入同一平台 artifact。
- UI 资源独立于原生后端入口；服务端按 `info.json.management_ui.entry` 校验入口，并从实际 artifact 目录读取静态文件。`artifact.json` v2 只描述目标平台和原生入口。
- contract 驱动的消息渲染模板继续位于 `templates/`，不使用 Vue。

```json
{
  "management_ui": {
    "entry": "ui/index.html",
    "pages": [
      {
        "id": "config",
        "label": "配置页面"
      }
    ]
  }
}
```

`management_ui.entry` 是所有页面共用、且必须实际存在于 artifact 中的相对 HTML 路径。`pages[].id` 是稳定页面标识，`label` 是宿主标题；当前页面 ID 通过 iframe 地址的 `page` 参数传递。

## 同源加载与 CSP

- 宿主以 iframe 嵌入 `/plugin-ui/{plugin_id}/{asset_path}`，该路径与管理面同源；资源只读取当前插件 artifact 的 `ui/` 目录，不提供目录枚举，不允许路径越界，也不做重定向。
- 插件页面沿用管理面的登录会话直接调用管理 API。插件是管理员确认安装的可信本地代码，同源页面不再与管理面隔离。
- 响应附带 `Content-Security-Policy`：`script-src` 只允许该插件 UI 路径，不允许内联脚本与 `eval`，并设置 `object-src 'none'` 与 `base-uri 'none'`；样式允许内联，图片允许 `data:`，网络请求只允许同源。`script-src` 按请求的 Host 生成，开发代理与反向代理需要原样转发浏览器访问的 Host。
- CSP 用于缓解两类风险：插件页从外部 CDN 加载的脚本被投毒，以及插件页把聊天内容当 HTML 渲染导致的 XSS。页面仍应以文本方式显示聊天内容。

## 设置与密钥

`@rayleabot/plugin-ui` 以当前会话调用插件范围内的接口，写操作沿用管理面的 CSRF 头：

| SDK | HTTP | 语义 |
| --- | --- | --- |
| `reloadSettings` | `GET /api/plugins/{plugin_id}/settings` | 读取当前设置 |
| `saveSettings` | `PUT /api/plugins/{plugin_id}/settings` | 覆盖非敏感设置 |
| `reloadSecretStatus` | `GET /api/plugins/{plugin_id}/secrets` | 只读取是否已配置 |
| `setSecrets` | `PUT /api/plugins/{plugin_id}/secrets` | 覆盖选定密钥，不回显明文 |
| `deleteSecrets` | `DELETE /api/plugins/{plugin_id}/secrets` | 显式删除选定密钥 |
| `invokeAction` | `POST /api/plugins/{plugin_id}/management/actions` | 把页面动作发送给所属插件 |

已保存的密钥明文不会出现在 GET 响应或其他网络回包中。运行中的插件可通过 `secret.read` local action 读取自身命名空间中的单个值。

设置按顶层键保存覆盖值；显式保存与默认值相同的值仍会形成持久化覆盖。`changed_keys` 只包含有效 JSON 值发生变化的键，按键排序。普通同值保存返回空数组，不刷新命令、不发送 `config.changed`。密钥批量替换和删除是原子操作；同值替换和删除不存在的密钥不计入改动，密钥内容不进入设置事件。

设置已保存而命令刷新或事件入队失败时，保存返回 `plugin.settings_apply_failed`，`details` 包含 `committed: true`、失败的 `stage` 和待应用的 `changed_keys`。再次保存相同值会继续处理待应用的改动，也可重载插件以读取完整已保存设置；错误不表示设置被回滚。

## 其他能力

- 页面可通过 `apiRequest` 调用其他管理接口，例如触发调度任务或读取指定 OneBot 实例的目标信息；这些请求同样使用当前会话与 CSRF 校验。
- 宿主按插件页面文档高度调整 iframe，范围为 320–1600px，并在视口内保留滚动空间。
- SDK 读取宿主页面的主题变量并映射为 `--raylea-*` CSS variables，宿主切换主题时同步更新。
- `trust.level = unverified` 的来源在首次打开、版本变化或来源变化后需要重新确认。

## 相关文档

- [Plugin Manifest](./manifest.md)
- [SDK](./sdk/README.md)
- [Management Surface](../user/management-surface.md)
