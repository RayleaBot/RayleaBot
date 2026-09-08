# RayleaBot Web 管理面工程基线

本文档定义 RayleaBot Web 管理面的当前正式工程边界。

## 基线结论

- Web 管理面使用 `Reka UI 2.10.4 + shadcn-vue 自有组件源码 + Tailwind CSS 4 + Motion for Vue 2.4.2`。
- 协议中心、应用壳、认证入口、插件、三方账号、治理、配置与诊断工作区统一使用产品组件。
- 应用源码、测试、构建配置和依赖位于 `web/`。
- 页头软件版本在 Vite 构建时由 `RAYLEA_BUILD_VERSION` 写入，接受 `dev` 或发行语义版本；未提供时显示“开发版本”。开发服务始终显示“开发版本”，正式发布流水线传入当前 tag。运行时配置、服务端状态与浏览器存储不参与版本选择。
- HTTP、WebSocket、会话和错误处理由共享请求模块与 Pinia stores 提供。

## 正式语义

HTTP API、WebSocket 事件、错误码、配置 schema、插件信息与协议以 [contracts](../../contracts/README.md) 为准。服务端是正式状态来源，页面通过请求结果和事件快照展示状态。

## 组件与页面结构

- 基础组件源码位于 `components/ui/`，来源、摘要与许可证随源码记录；业务页面使用 `AppButton`、`AppField`、`AppSelect`、`AppDialog` 等产品封装。
- 独立单值字段通过 `AppField floating` 选择浮动标签。字段上下文将该呈现方式传给 `AppInput`、`AppNumberInput`、`AppTextarea` 和 `AppSelect`，保留同一个原生 label、控件 ID、必填状态及错误／说明关联；页面不单独覆盖标签位置。复合字段、多选、开关和配置说明行保留外置标签。
- `AppQRCode` 使用 `qrcode-generator 2.0.4` 编码登录链接，UTF-8 转换、静区与扫描图像颜色由编码边界维护；状态、刷新操作和提示使用产品组件。版权与 MIT 文本由版本化补充文件纳入发布 notices。
- Reka 负责交互语义与焦点，Motion 负责进入、退出和内容尺寸变化。弹窗在动画完成后释放交互节点和遮罩，位置始终由 CSS 视口居中计算。
- 左右抽屉复用弹窗的退出和焦点生命周期，保持完整视口高度；仅居中弹窗对正文高度插值。菜单和提示使用统一的语义时长，关闭菜单进入弹窗时由浮层层级保证新任务在上方。
- 菜单、选择器、页签和分段选择交由 Reka 处理键盘与选中语义；连续导航取消正在执行的页面与主题动画，降级动画从 Motion for Vue 导出入口调用。
- 标签输入保留业务要求的分隔符与值类型，多选允许清除当前筛选；数据表使用原生表格并在自己的区域横向滚动。需要保留控制台或 iframe 的页签显式开启常驻内容。
- 移动插件筛选使用贴底抽屉，正文受视口高度约束；安装检查和可信代码确认在浮层退出完成后清理显示数据。
- 桌面日志详情使用受宿主尺寸约束的非模态浮窗，支持拖动位置记忆和列表滚动锚点；窄屏使用右侧抽屉。展示层保留退出所需内容，详情控制器维护选择状态和请求。
- 调度详情复用居中弹窗，宽表保留固定操作列。模板 iframe 在已居中的缩放容器内以左上角为原点缩放，调整视口不重新创建预览文档。
- `/__dev/components` 仅在开发构建注册，使用正式管理会话，不加入生产菜单或产物。
- 页面壳、菜单、页签、面包屑、主题偏好和工作区身份由布局、路由与 `ui-shell` store 维护。
- `stores/`、`lib/`、`views/` 与 `components/` 分别承担业务状态、共享逻辑、页面和组件职责。
- `AppPage` 统一标题、说明、状态、主操作、工具栏、内容宽度和全高工作区；`AppCard` 只包含真实独立表面或无阴影分区。
- `AppStatusTag`、`RetryPanel`、`AppEmptyState`、`ManagementContextActions`、共享日志筛选与详情抽屉、模板预览工作区作为正式业务组件。

## 目录与职责

| 路径 | 职责 |
| --- | --- |
| `layouts/` | 页面壳、菜单、页头、页签、面包屑 |
| `router/` | 路由表与正式页面注册 |
| `adapter/` | 反馈、运行时桥接和 UI 层薄适配 |
| `request/` / `lib/http.ts` | HTTP 请求、鉴权、下载和错误信封解析 |
| `lib/ws.ts` | 受控 WebSocket 连接封装 |
| `stores/` | Pinia stores、工作区状态和实时快照 |
| `access/` | 路由准入与会话驱动可达性 |
| `preferences/` | 主题、布局和显示偏好 |
| `styles/` | Tailwind 入口、SCSS 分区和生成主题 token 转发 |
| `types/` | contract 生成类型的别名与共享类型 |
| `locales/` | zh-CN 文案资源 |
| `views/` | 页面与页面级工作区 |

## 请求与实时通信

- HTTP 请求提供：
  - 浏览器会话使用 Host-only HttpOnly cookie 与 `X-Raylea-CSRF` 请求头，CSRF 值只保存在内存
  - 请求超时
  - `401` 时清理内存会话快照并回到登录入口
  - RayleaBot error envelope 解析
  - 下载文件名解析
- Bearer transport 用于非浏览器客户端；Web 会话初始化时清理本地存储中的 bearer token。
- WebSocket 使用受控连接模型，覆盖：
  - `events`
  - `logs`
  - `pluginConsole`
- Dashboard 在事件连接正常时使用事件驱动刷新状态；手动刷新和断线回退继续使用 HTTP。

## 工作区与 keep-alive 规则

- `commands`、`logs`、`logs-history` 和 `render-templates` 使用稳定 `viewKey`，在 query 变化时复用同一个工作区实例和同一个页签。
- `plugin-detail` 保持按插件 ID 独立详情页签。
- 工作区 query 只表达当前筛选、选中项和详情抽屉状态，不制造重复页签和历史噪音。
- 模板预览页使用 `/render/templates/:templateId?` 单页工作区，模板切换使用同一页面实例。
- 桌面端只有在打开多个工作区时显示页签，移动端隐藏页签；页签隐藏不改变 keep-alive、搜索跳转和工作区恢复语义。
- 偏好持久化版本为 `3`，包含主题、密度、内容宽度、页面动效、页签和工作区记忆。

## 当前正式页面

- 登录、初始化和会话入口
- 离线状态异常页
- 系统状态
- 菜单中心、插件列表、插件商店、全局插件设置、插件详情和指令中心
- 三方账号、协议中心和兼容矩阵
- 权限策略、黑白名单和限流中心
- 定时任务、实时日志和历史日志
- 配置和模板预览

## 页面联动基线

- 仪表盘、协议中心、日志中心、插件详情、指令中心和模板预览之间通过稳定字段跳转。
- 日志详情使用 `plugin_id`、`protocol`、`request_id` 生成上下文入口；任务更新以 `source=tasks` 日志进入同一日志工作区。
- 协议中心提供兼容矩阵入口，并可进入日志中心的实时日志页，自动带上 `protocol=onebot11` 筛选。
- 插件详情提供当前插件的指令中心和历史日志入口。

## 样式与组件映射

- 产品组件将 Tailwind 语义别名映射至共享 CSS Variables，保留 SCSS 分区。
- 样式入口为 `src/main.ts` 引入的 `@/styles/tailwind.css` 与 `@/styles/main.scss`；生成主题 token 经 `styles/_tokens.scss` 转发的 `theme-tokens.generated` 消费。
- `system`、`light`、`dark` 主题消费同一套生成 CSS Variables；系统主题变化只影响 `system` 模式。
- 状态色调统一为 `neutral`、`info`、`success`、`warning`、`attention`、`danger`，未知状态回退为中性。
- 工作区统一使用产品表单与关闭机制，不在页面内另建一套组件或浮层行为。
- 普通对象列表在窄屏使用摘要行；兼容矩阵、名单、代码和技术字段允许局部横向滚动，普通页面不得横向溢出。

## 验证门禁

- `pnpm build`
- `pnpm test`
- `pnpm test:e2e`

## 约束

- 不新增平行 HTTP client、WebSocket client、状态管理或组件系统。
- 不在前端发明 contract 外字段、状态名或错误码。
- 不通过解析日志推断真实状态。
- 不把 Web 改成 Launcher 的子状态来源。

## 官方参考

- [Reka UI](https://www.reka-ui.com/docs/overview/introduction)
- [shadcn-vue](https://www.shadcn-vue.com/docs/introduction)
- [Motion for Vue](https://motion.dev/docs/vue)
