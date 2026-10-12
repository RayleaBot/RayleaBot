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

`management_ui.entry` 是所有页面共用、且必须实际存在于 artifact 中的相对 HTML 路径。`pages[].id` 是稳定页面标识，`label` 是宿主标题，`scope` 是页面范围，见[按机器人账号的页面](#按机器人账号的页面)；当前页面 ID 通过 iframe 地址的 `page` 参数传递。

## 同源加载与 CSP

- 宿主以 iframe 嵌入 `/plugin-ui/{plugin_id}/{asset_path}`，该路径与管理面同源；资源只读取当前插件 artifact 的 `ui/` 目录，不提供目录枚举，不允许路径越界，也不做重定向。
- 插件页面沿用管理面的登录会话直接调用管理 API。插件是管理员确认安装的可信本地代码，同源页面不再与管理面隔离。
- 响应附带 `Content-Security-Policy`：`script-src` 只允许该插件 UI 路径，不允许内联脚本与 `eval`，并设置 `object-src 'none'` 与 `base-uri 'none'`；样式允许内联；图片与音视频允许同源、`data:` 和任意 HTTPS 来源；字体与网络请求只允许同源。`script-src` 按请求的 Host 生成，开发代理与反向代理需要原样转发浏览器访问的 Host。通过 IPv6 字面地址（如 `http://[::1]:8080`）访问时，CSP 无法写出该地址，`script-src` 回退为 `'self'`：外部脚本与内联脚本仍被拒绝，但不再限定到本插件路径。
- CSP 不用于约束插件作者：插件是可信本地代码，后端本就能自行访问网络。它约束的是同源页面里的第三方内容。插件页显示的群名、昵称、聊天和订阅内容来自外部，一旦被当作 HTML 渲染，注入的脚本就能以管理员会话调用管理 API；从外部 CDN 加载的脚本也可能被投毒。因此脚本只允许插件自身路径，网络请求只允许同源。图片与音视频不执行代码，允许 HTTPS 来源以显示头像和封面；`Referrer-Policy: same-origin` 使这些跨域请求不携带管理面地址。页面仍应以文本方式显示外部内容。

## 设置与密钥

`@rayleabot/plugin-ui` 以当前会话调用插件范围内的接口，写操作沿用管理面的 CSRF 头：

| SDK | HTTP | 语义 |
| --- | --- | --- |
| `reloadSettings` | `GET /api/plugins/{plugin_id}/settings` | 读取当前设置 |
| `saveSettings` | `PUT /api/plugins/{plugin_id}/settings` | 覆盖非敏感设置 |
| `reloadSecretStatus` | `GET /api/plugins/{plugin_id}/secrets` | 只读取是否已配置 |
| `setSecrets` | `PUT /api/plugins/{plugin_id}/secrets` | 覆盖选定密钥，不回显明文 |
| `deleteSecrets` | `DELETE /api/plugins/{plugin_id}/secrets` | 显式删除选定密钥 |
| `invokeAction` | `POST /api/plugins/{plugin_id}/management/actions` | 把页面动作发送给所属插件；按账号的页面自动附带页面与账号 |
| `reloadBotAccount` | `GET /api/bot-accounts/{source_protocol}/{bot_id}` | 读取当前账号的资料、所在连接与状态 |

已保存的密钥明文不会出现在 GET 响应或其他网络回包中。运行中的插件可通过 `secret.read` local action 读取自身命名空间中的单个值。

设置按顶层键保存覆盖值；显式保存与默认值相同的值仍会形成持久化覆盖。`changed_keys` 只包含有效 JSON 值发生变化的键，按键排序。普通同值保存返回空数组，不刷新命令、不发送 `config.changed`。密钥批量替换和删除是原子操作；同值替换和删除不存在的密钥不计入改动，密钥内容不进入设置事件。

设置已保存而命令刷新或事件入队失败时，保存返回 `plugin.settings_apply_failed`，`details` 包含 `committed: true`、失败的 `stage` 和待应用的 `changed_keys`。再次保存相同值会继续处理待应用的改动，也可重载插件以读取完整已保存设置；错误不表示设置被回滚。

## 按机器人账号的页面

`pages[].scope` 声明页面范围：

- `global`：缺省值，页面管理插件级的设置与数据。
- `bot_account`：页面管理归属某个机器人账号的数据，例如该账号的订阅、群设置或提醒。

```json
{ "id": "subscriptions", "label": "订阅设置", "scope": "bot_account" }
```

宿主在 `bot_account` 页面上方显示账号切换栏，账号来自服务端的机器人账号目录。一个账号同一时间只登录在一条连接上，切换栏按“连接名 · 昵称（账号）”列出账号；连接被删除或改登其他账号后，该账号标为无连接，仍可选择，它的数据不会消失。当前账号以 `bot=<source_protocol>:<bot_id>` 写在 iframe 地址中，账号部分按原样使用；切换账号时宿主用新地址重新加载页面。

`@rayleabot/plugin-ui` 的 `usePluginHost` 提供：

- `page`：页面 ID 与范围。
- `bot`：当前账号在目录中的资料，包括昵称、当前连接与在线状态；`global` 页面为 `null`。资料可能在页面打开期间变化，只用于显示。
- `refreshBot()`：重新读取账号资料，例如在需要当前连接的在线查询之前；账号本身在页面生命周期内不变。
- `client.invokeAction`：在 `bot_account` 页面自动附带 `page_id` 与 `bot`。宿主核对页面已声明为按账号、账号在目录中（离线或无连接也可以）后，把两者写入 `management.action` 事件，插件后端用 Go SDK 的 `event.Event.ManagementBot()` 读取。`global` 页面不附带。

页面范围与账号选择不是权限隔离：同源页面和已登录的客户端可以自行构造任意请求，包括省略 `page_id` 的请求。插件后端需要：

- 按事件中的账号过滤读取、新建、修改、删除与批量操作；按记录 ID 修改或删除时核对记录属于该账号，账号缺失时拒绝需要账号的动作。
- 账号页的写入只合并当前账号的数据。不能在页面上过滤后用 `saveSettings` 回写整份设置，否则会清掉其他账号的数据。
- 页面的缓存、草稿与未完成请求按账号区分；切换账号后不复用旧账号的结果。

账号离线或无连接时，页面仍可读取和编辑已保存的数据；需要在线连接的查询（例如群列表）提示不可用，不能把查询不到当作清空数据。账号从目录移除后，它的账号页不能再打开，插件应在 `global` 页面提供查看和清理这些记录的入口；同一账号重新登录后会重新进入目录，原有数据的归属不变。

这个入口按记录保存的协议和账号列出数据，后端只需返回自己记录的账号，不保存目录副本；页面用 `apiRequest` 调用 `GET /api/bot-accounts/{source_protocol}/{bot_id}` 逐个确认，返回 404 才算账号已不在目录中，不能以“不在已加载的列表里”判断。缺少账号的旧记录也集中在这里处理，不在每个账号页重复显示：只有协议和目标身份仍然有效时，才允许使用者把记录显式归入目录中的某个账号；QQ 官方的 openid 只在原机器人下有效，不能改挂到另一账号；带完整账号的目录外记录不能当作未分配记录重新认领。

## 其他能力

- 页面可通过 `apiRequest` 调用其他管理接口，例如触发调度任务或读取指定 OneBot 实例的目标信息；这些请求同样使用当前会话与 CSRF 校验。
- 宿主让 iframe 填满管理工作区的可用高度，并随窗口与页头布局变化自动伸缩；内容较少时仍保持全高，内容超出时在插件页面内滚动。
- SDK 读取宿主页面的主题变量并映射为 `--raylea-*` CSS variables，宿主切换主题时同步更新。
- `trust.level = unverified` 的来源在首次打开、版本变化或来源变化后需要重新确认。

## 相关文档

- [Plugin Manifest](./manifest.md)
- [SDK](./sdk/README.md)
- [Management Surface](../user/management-surface.md)
