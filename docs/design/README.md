# Design

本目录介绍 RayleaBot 产品界面的视觉、交互与分面映射规范。根目录 [`PRODUCT.md`](../../PRODUCT.md) 说明产品语境，根目录 [`DESIGN.md`](../../DESIGN.md) 定义项目级视觉规范，本目录说明各产品界面如何在固定的技术栈上应用这些语义。

HTTP、WebSocket、schema、错误码、插件协议和发布元数据以 [`contracts/`](../../contracts/README.md) 为准。设计文档不新增接口字段、状态名或客户端状态来源。

## 阅读顺序

1. [`PRODUCT.md`](../../PRODUCT.md)：用户、产品目的、品牌性格、反例与无障碍目标。
2. [`DESIGN.md`](../../DESIGN.md)：颜色、字体、层次、组件与强制视觉规则。
3. 本目录分面文档：Web、Launcher 和插件管理页的组件体系映射。
4. 当前实现：各分面在采用矩阵中的状态与验收入口。

## 分面文档

| 文档 | 作用 |
| --- | --- |
| [Web Management UI](./web-management-ui.md) | Reka UI、自有 shadcn-vue 组件、应用壳与页面组合规范 |
| [Launcher Design System](./launcher-design-system.md) | Fluent UI React v9、Wails 桌面壳与本机操作规范 |
| [Plugin Management Surface](./plugin-management-surface.md) | 官方插件页面完整规范与第三方页面兼容包络 |

## 采用矩阵

| 分面 | 状态 | 当前实现边界 | 采用完成条件 |
| --- | --- | --- | --- |
| 设计上下文 | `documented` | `PRODUCT.md`、`DESIGN.md` 与 `.impeccable/design.json` 提供战略、视觉和扩展元数据 | loader 能同时读取产品与设计上下文，设计文件由 CI 识别为 docs |
| Web 管理面 | `adopted` | 管理壳、认证入口和全部正式工作区已映射项目级语义；主题支持 `system`、`light`、`dark`，桌面多工作区显示页签，移动端使用抽屉导航与摘要行 | 后续变更沿用语义 token、状态色调、异常优先披露和响应式结构，并满足 Web 分面验收条件 |
| Launcher | `adopted` | Fluent theme、CSS variables、原生窗口背景与五个工作区已采用项目级冷暖语义；`760×560` 最小窗口使用顶部紧凑导航 | 后续变更沿用语义 token、主题同步、桌面壳职责，并满足 Launcher 分面验收条件 |
| 官方插件 | `adopted` | 独立插件仓库使用 Vue 3、TypeScript、Vite、按需 Ant Design Vue 与 `@rayleabot/plugin-ui`；产物自带运行时和样式 | 后续变更沿用 bridge v3、主题 token、焦点、状态和窄屏行为 |
| 第三方插件 | `compatible-envelope` | 宿主负责 iframe 边界、载入状态、安全确认和错误恢复；页面不继承宿主全局样式 | 页面支持系统主题媒体查询、键盘操作、对比度和 reduced-motion，不要求使用 RayleaBot 组件 |
| 宿主主题同步 | `adopted` | bridge v3 的 `host.init` 下发亮暗模式和允许的设计 token；SDK 将其映射到插件页面根节点 | 后续字段先进入 bridge contract，再同步 fixture、宿主、Vue SDK、官方插件和文档 |

`documented` 表示正式文档与机器可读上下文完整，`adopted` 表示运行时已采用并由对应分面测试约束，`compatible-envelope` 表示插件可独立实现页面，不要求使用 RayleaBot 组件。

## 共同边界

- Web 使用 Reka UI、自有 shadcn-vue 组件、Tailwind CSS 与 Motion for Vue。
- Launcher 使用 Fluent UI React v9，并通过生成的 Wails bindings 连接 Go 桌面宿主。
- 插件管理页使用包内静态 HTML、CSS 和 JavaScript 产物，不依赖宿主组件运行时；官方插件的这些产物由 Vue 3、TypeScript 和 Vite 构建。
- 三个分面共享颜色角色、字体层级、间距、圆角、状态语义和无障碍门槛，不共享框架组件。
- 聊天图片模板属于渲染产物，不受本产品界面规范约束。

## 设计工具

Impeccable 的项目上下文由根目录 `PRODUCT.md` 和 `DESIGN.md` 提供。Web 与 Launcher 各有独立的 pnpm 工作区；从仓库根设置 `IMPECCABLE_CONTEXT_DIR`，使指定应用目标的命令读取共享上下文：

```powershell
$env:IMPECCABLE_CONTEXT_DIR = (Get-Location).Path
& '.agents/skills/impeccable/scripts/impeccable.cmd' context --target web/src
```

共享设计记录的维护在仓库根执行 `.agents/skills/impeccable/scripts/impeccable.cmd doctor --json`。设计稿、评审截图、交互会话和构建草稿在本地使用；`PRODUCT.md`、`DESIGN.md`、`design/tokens.json` 与 `.impeccable/design.json` 随仓库维护。
