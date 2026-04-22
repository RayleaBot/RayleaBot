# RayleaBot Web 管理面工程基线：Ant Design Vue + Vue Vben Admin

本文档定义 RayleaBot Web 管理面的当前正式工程边界。

## 基线结论

- Web 管理面采用 `Ant Design Vue 4.2.6 + Vue Vben Admin 5.7.0` 对齐方案。
- 实现范围固定在 `web/` 单应用内，不拆成官方整仓 `monorepo` 或 `turbo` 结构。
- 对外 HTTP API、WebSocket 事件、错误码、配置 schema 和外部类型保持不变。
- HTTP、WebSocket、会话和错误信封继续复用现有 RayleaBot 语义，不引入第二套状态源。

## Contract Audit

- 当前前端基线不修改以下正式契约：
  - `contracts/web-api.openapi.yaml`
  - `contracts/websocket-events.yaml`
  - `contracts/error-codes.yaml`
  - `contracts/config.user.schema.json`
  - `contracts/plugin-info.schema.json`
  - `contracts/plugin-protocol.schema.json`
- 后续若需要新的管理读取面、菜单资源或鉴权字段，继续按 contract-first 处理。

## 当前工程落点

- 组件层统一使用 Ant Design Vue。
- 页面壳、菜单、页签、面包屑、主题偏好和工作区行为按 Vben 风格组织。
- 现有业务语义保留在 `stores/`、`lib/`、`views/` 与 `components/` 内，不重定义后端 contract。
- `AppCard`、`AppPage`、`AppEmptyState`、`ManagementContextActions`、日志详情抽屉、模板编辑工作区和恢复卡片作为当前正式业务组件。

## 目录与职责

| 路径 | 职责 |
| --- | --- |
| `layout/` | 页面壳、菜单、页头、页签、面包屑 |
| `adapter/` | 反馈、运行时桥接和 UI 层薄适配 |
| `request/` / `lib/http.ts` | HTTP 请求、鉴权、下载和错误信封解析 |
| `lib/ws.ts` | 受控 WebSocket 连接封装 |
| `stores/` | Pinia stores、工作区状态和实时快照 |
| `access/` | 路由准入与会话驱动可达性 |
| `preferences/` | 主题、布局和显示偏好 |
| `views/` | 页面与页面级工作区 |

## 请求与实时通信

- HTTP 请求继续保留：
  - `Authorization: Bearer <token>`
  - 请求超时
  - `401` 时按 token 快照清理会话
  - RayleaBot error envelope 解析
  - 下载文件名解析
- WebSocket 继续使用受控连接模型，覆盖：
  - `events`
  - `tasks`
  - `logs`
  - `pluginConsole`
- Dashboard 在事件连接正常时使用事件驱动刷新状态；手动刷新和断线回退继续使用 HTTP。

## 工作区与 keep-alive 规则

- `commands`、`tasks`、`logs`、`logs-history` 和 `render-templates` 使用稳定 `viewKey`，在 query 变化时复用同一个工作区实例和同一个页签。
- `plugin-detail` 保持按插件 ID 独立详情页签。
- 工作区 query 只表达当前筛选、选中项和详情抽屉状态，不制造重复页签和历史噪音。
- 模板编辑页使用 `/render/templates/:templateId?` 单页工作区，模板切换使用同一页面实例。

## 当前正式页面

- 登录、初始化和会话入口
- 系统状态
- 插件与插件详情
- 指令中心
- 任务
- 实时日志与历史日志
- 协议中心与兼容矩阵
- 配置
- 模板编辑

## 页面联动基线

- 仪表盘、协议中心、日志中心、任务、插件详情、指令中心和模板编辑器之间通过稳定字段跳转。
- 日志详情使用 `plugin_id`、`protocol`、`request_id` 生成上下文入口。
- 任务详情使用 `plugin_id`、`protocol`、`request_id`、`template` 生成上下文入口。
- 协议中心提供兼容矩阵入口，并可进入日志中心的实时日志页，自动带上 `protocol=onebot11` 筛选。
- 插件详情提供当前插件的指令中心和历史日志入口。

## 样式与组件映射

- 样式系统采用 Ant Design Vue tokens、Tailwind CSS 4、SCSS 和 CSS Variables。
- 表单、表格、弹窗、抽屉、空态、骨架屏、标签和消息提示统一使用 Ant Design Vue 对应组件。
- 不保留 `element-plus`、`ElMessage` 和 `.el-*` 样式选择器。

## 验证门禁

- `pnpm build`
- `pnpm test`
- `pnpm test:e2e`
- `rg -l "element-plus|<el-|ElMessage" web/src web/tests/unit`

## 约束

- 不新增平行 HTTP client、WebSocket client、状态管理或组件系统。
- 不在前端发明 contract 外字段、状态名或错误码。
- 不通过解析日志推断真实状态。
- 不把 Web 改成 Launcher 的子状态源。

## 官方参考

- [Vben Quick Start](https://doc.vben.pro/guide/introduction/quick-start.html)
- [About Vben Admin](https://doc.vben.pro/guide/introduction/vben.html)
- [UI Framework Switching](https://doc.vben.pro/en/guide/in-depth/ui-framework.html)
- [Styles](https://doc.vben.pro/guide/essentials/styles.html)
- [Directory Explanation](https://doc.vben.pro/en/guide/project/dir.html)
- [Vben Form](https://doc.vben.pro/components/common-ui/vben-form.html)
- [Ant Design Vue README](https://github.com/vueComponent/ant-design-vue)
