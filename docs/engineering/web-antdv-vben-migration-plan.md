# RayleaBot Web 管理面迁移计划：Ant Design Vue + Vue Vben Admin

本文档定义 RayleaBot Web 管理面的正式迁移方案。目标是把现有前端技术栈从 Element Plus 收口到 `Ant Design Vue + Vue Vben Admin` 对齐方案，同时保持 RayleaBot 现有 HTTP API、WebSocket、错误码、配置 schema 和外部类型不变。

## 迁移结论

- Web 管理面采用 `Ant Design Vue 4.2.6 + Vue Vben Admin 5.7.0` 对齐方案作为正式目标基线。
- 迁移只发生在 `web/` 工程内部；仓库整体继续保持当前单仓结构，不改成 Vben 官方整仓 `monorepo` / `turbo`。
- 已正式发布并可锁版本的 `@vben/*` 包直接纳入依赖；未公开发布的 Vben 能力按官方实现移植到仓库维护。
- HTTP 层收口到 Vben request 风格封装，继续保留 RayleaBot 现有 `Authorization`、超时、`401` 清理会话和错误包解析语义。
- WebSocket 继续沿用当前受控连接模型，不引入第二套实时通信栈。
- 样式系统采用 Ant Design Vue tokens + Vben 样式体系 + Tailwind CSS 4 + SCSS + CSS Variables。
- 当前仓库固定 Node.js `24.14.0`，满足 Vben 官方 `20.15.0+` 的最低要求。

## Contract Audit 结论

- 本轮 contract audit 结论：**无 formal contract 变更**。
- 本轮不修改以下正式边界：
  - `contracts/web-api.openapi.yaml`
  - `contracts/websocket-events.yaml`
  - `contracts/error-codes.yaml`
  - `contracts/config.user.schema.json`
  - `contracts/plugin-info.schema.json`
  - `contracts/plugin-protocol.schema.json`
- 如果后续为了 Vben 菜单、权限、主题或资源管理引入新的后端读取面，必须作为独立的 contract-first 变更处理。

## 当前基线

- `web/src` 与 `web/tests/unit` 当前共有 38 个文件直接引用 `Element Plus`。
- 当前代码中约有 549 处 `Element Plus` 相关引用，覆盖布局、按钮、表单、消息提示、表格、弹窗、抽屉、空态、标签和骨架屏。
- 当前 `web/` 已存在稳定的路由、Pinia、认证、HTTP、WebSocket 和页面分层，这些能力迁移时继续保留业务语义，不重定义后端 contract。

## 来源策略

### 直接纳入并锁版本的依赖

- `ant-design-vue@4.2.6`
- `@vben/request@1.0.1`
- `@vben/layouts@1.0.1`
- `@vben/stores@1.0.1`
- `tailwindcss@4.x`

### 作为官方骨架参考并移植到仓库维护的能力

- `preferences`
- `access`
- `common-ui`
- `plugins`
- 其他未公开发布、但 Web 管理面需要的 Vben 页面骨架与偏好实现

### 不采用的来源方式

- Git 子模块
- Git URL 直连依赖
- 将 Vben 官方整仓并入当前仓库
- 在 `web/` 外新增平行模板工程

## 目录目标

迁移完成后，`web/src` 的目标分层固定为：

- `layout/`
  - 页面壳、侧栏、页头、面包屑、全局 provider 入口
- `adapter/`
  - Vben 与 Ant Design Vue 的桥接层
  - 现有业务组件到新组件体系的薄适配
- `request/`
  - HTTP client、鉴权、错误处理、统一 loading、下载行为
- `stores/`
  - Pinia store 与 Vben stores 对齐组织
- `access/`
  - 路由准入、页面级权限入口、会话驱动的菜单与页面可达性
- `preferences/`
  - 主题、布局偏好、展示密度、导航状态
- `views/`
  - 业务页面与页面级组合组件

当前目录向目标目录的收口方向固定如下：

| 当前位置 | 目标方向 |
| --- | --- |
| `web/src/pages/*` | `web/src/views/*` |
| `web/src/components/AppShell.vue` | `web/src/layout/` 下的主应用壳 |
| `web/src/lib/http.ts` | `web/src/request/` 下的统一请求入口 |
| `web/src/lib/ws.ts` | `web/src/adapter/` 或实时连接子层 |
| `web/src/stores/*` | 保留领域边界，按 Vben stores 方式重组 |

## 组件映射

| Element Plus | 迁移目标 |
| --- | --- |
| `el-button` | `a-button` |
| `el-form` / `el-form-item` | `a-form` / Vben Form adapter |
| `el-input` | `a-input` |
| `el-input-number` | `a-input-number` |
| `el-select` / `el-option` | `a-select` |
| `el-switch` | `a-switch` |
| `el-checkbox` / `el-checkbox-group` | `a-checkbox` / `a-checkbox-group` |
| `el-table` / `el-table-column` | `a-table` / Vben table 方案 |
| `el-dialog` | `a-modal` |
| `el-drawer` | `a-drawer` |
| `el-alert` | `a-alert` |
| `el-tag` | `a-tag` |
| `el-empty` | `a-empty` |
| `el-skeleton` / `el-skeleton-item` | `a-skeleton` |
| `el-card` | `a-card` |
| `el-descriptions` / `el-descriptions-item` | `a-descriptions` |
| `ElMessage` | Ant Design Vue `message` |

## 执行顺序

### 1. 依赖与基础壳

目标：

- 完成 `ant-design-vue`、Vben 已发布包、Tailwind、主题入口和全局 provider 准入。

固定动作：

1. 更新 `web/package.json` 与 `web/pnpm-lock.yaml`。
2. 引入 Ant Design Vue 样式入口和 Vben 对齐所需样式入口。
3. 接入 Tailwind CSS 4，同时保留现有 SCSS 管线。
4. 在 `main.ts` 注册新的 UI provider、主题和全局样式入口。
5. 建立新的 `layout / adapter / request / stores / access / preferences / views` 目录骨架。

验收：

- 开发与构建入口不变，`pnpm build` 能通过。
- 全局样式入口稳定，不引入第二套命令或第二套 `web/` 应用。

### 2. 应用骨架

目标：

- 用 Vben 风格应用壳替换当前 Web 页面骨架，但保留现有路由语义。

固定动作：

1. 以现有 `RouterView` 与路由树为基础，重建 `App.vue`、主布局壳、菜单、页头、内容区与偏好入口。
2. 迁移 `AppShell` 的导航、页头、系统状态提示和关闭动作到新布局。
3. 保留当前路由名、路径、登录态守卫和 `titleKey` 语义。
4. 菜单与面包屑以现有路由定义为唯一来源，不新增后端读取面。

验收：

- 登录前后页面跳转规则保持不变。
- 菜单、页面标题和当前路由语义与现有实现一致。

### 3. 数据层

目标：

- 把 HTTP 与全局请求状态组织收口到新的 request 层，同时不改变 RayleaBot 现有后端语义。

固定动作：

1. 将现有 `apiRequest` / `apiDownload` 语义迁移到新的 request client。
2. 保留以下行为：
   - `Authorization: Bearer <token>`
   - 请求超时
   - `401` 时按 token 快照清理会话
   - 解析 RayleaBot error envelope
   - 下载文件名解析
3. 保留现有 WebSocket 管理类的状态机和重连行为。
4. 按 Vben stores 对齐方式重排 Pinia store 入口，不改变 store 的后端输入输出 shape。

验收：

- 认证、下载、错误显示、会话失效和 4 条 WebSocket 连接行为全部保持一致。
- 不引入第二套 HTTP client 或第二套 WebSocket client。

### 4. 表单与消息层

目标：

- 完成消息提示、表单、抽屉、弹窗、空态和骨架屏的统一切换。

固定动作：

1. 用 Ant Design Vue `message` 替换全部 `ElMessage`。
2. 用 `a-form` / Vben Form adapter 替换登录、初始化、配置、筛选和权限弹窗表单。
3. 配置页面采用 `Ant Design Vue Form + ajv` 双层校验：
   - 表单即时反馈使用字段规则
   - 提交前最终校验使用 `contracts/config.user.schema.json`
4. 用 `a-modal` / `a-drawer` 替换对话框与抽屉。
5. 用 `a-table` / Vben table 方案替换表格。
6. 用 `a-alert`、`a-tag`、`a-empty`、`a-skeleton` 替换状态型组件。

验收：

- 所有反馈文案、确认路径和空态语义与当前页面一致。
- 配置保存和错误反馈不引入新的 contract 外字段。

### 5. 页面迁移

迁移顺序固定如下：

1. `Login`
2. `Setup`
3. `AppShell`
4. `Dashboard`
5. `Config`
6. `Plugins`
7. `PluginDetail`
8. `Commands`
9. `Tasks`
10. `Logs`
11. `Protocols`
12. `ProtocolLogs`

页面级固定要求：

- 任何页面迁移都不得改变既有 API 路径、请求参数、WebSocket 事件或错误码。
- 页面迁移完成前，允许局部兼容层存在；兼容层只承担桥接职责，不承担新的业务状态源职责。
- 页面级视觉结构可以对齐 Vben 后台壳，但业务流程、字段含义和操作边界必须与现有 contract 一致。

### 6. 测试迁移

目标：

- 把测试基础设施、挂载方式和 mock 从 Element Plus 迁移到新骨架。

固定动作：

1. 更新 `web/tests/unit/main.spec.ts` 的启动 mock。
2. 把 `ElementPlus` 挂载改为 Ant Design Vue / 新 provider 挂载。
3. 更新针对 `ElMessage` 的 mock 为 Ant Design Vue `message`。
4. 调整单测中的组件名查找和页面结构断言。
5. 保留 E2E 的核心业务断言范围，必要时补稳定的 `data-testid`。

验收：

- `pnpm test` 与 `pnpm test:e2e` 全通过。
- 单测和 E2E 继续覆盖登录、初始化、配置、插件、任务、日志和协议中心主链。

### 7. 清理收口

目标：

- 删除旧依赖、旧样式和迁移兼容层。

固定动作：

1. 删除 `element-plus` 依赖和相关 CSS 入口。
2. 删除 `.el-` 选择器、旧样式变量和旧兼容桥接。
3. 清理 `Element Plus` 专用测试 mock。
4. 清理只服务旧骨架的临时目录和过渡适配。

最终移除条件：

- `pnpm build`、`pnpm test`、`pnpm test:e2e` 全通过。
- `rg -l "element-plus|<el-|ElMessage" web/src web/tests/unit` 结果为 0。

## 回退策略

- 文档冻结阶段不改代码，可直接回滚。
- 实施阶段按页面批次切换，每一批都保持主分支可构建、可测试。
- 在 `element-plus` 依赖完全清零前，允许存在受控兼容层。
- 如果某一批次导致关键主链回归失败，直接回退该批次，不带着未完成迁移进入下一批。

## 实施验收

### 文档阶段

- 基线文档、治理规则、执行计划和技术评估的现行口径已收口到 `Ant Design Vue + Vue Vben Admin`。
- 本文档可独立指导后续实施，不留实现者自行裁决的关键空白。

### 代码阶段

- `web/package.json` 与 `web/pnpm-lock.yaml` 完成新依赖冻结。
- `pnpm build`、`pnpm test`、`pnpm test:e2e` 全通过。
- `rg -l "element-plus|<el-|ElMessage" web/src web/tests/unit` 结果为 0。
- 登录、初始化、会话过期、4 条 WebSocket、配置保存、插件管理、任务详情、日志过滤、协议页和恢复 / 诊断主链行为与当前 contract 一致。

## 官方依据

- [Vben Quick Start](https://doc.vben.pro/guide/introduction/quick-start.html)
- [About Vben Admin](https://doc.vben.pro/guide/introduction/vben.html)
- [UI Framework Switching](https://doc.vben.pro/en/guide/in-depth/ui-framework.html)
- [Styles](https://doc.vben.pro/guide/essentials/styles.html)
- [Project Update](https://doc.vben.pro/en/guide/other/project-update.html)
- [Directory Explanation](https://doc.vben.pro/en/guide/project/dir.html)
- [Vben Form](https://doc.vben.pro/components/common-ui/vben-form.html)
- [Ant Design Vue README](https://github.com/vueComponent/ant-design-vue)
