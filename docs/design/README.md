# Design

本目录说明 Web、Launcher 和插件管理页的视觉与交互规范。根目录 [`PRODUCT.md`](../../PRODUCT.md) 说明产品语境，[`DESIGN.md`](../../DESIGN.md) 定义共享视觉规范，各界面根据固定技术栈应用这些规则。

HTTP、WebSocket、schema、错误码、插件协议和发布元数据以 [`contracts/`](../../contracts/README.md) 为准。设计文档不新增接口字段、状态名或客户端状态来源。

## 阅读顺序

1. [`PRODUCT.md`](../../PRODUCT.md)：用户、产品目的、品牌性格、反例与无障碍目标。
2. [`DESIGN.md`](../../DESIGN.md)：颜色、字体、层次、组件与强制视觉规则。
3. 本目录各界面规范：Web、Launcher 和插件管理页的组件与交互规则。
4. 当前实现：各界面在采用矩阵中的状态与验收入口。

## 各界面规范

| 文档 | 作用 |
| --- | --- |
| [Web Management UI](./web-management-ui.md) | Reka UI、自有 shadcn-vue 组件、应用壳与页面组合规范 |
| [Launcher Design System](./launcher-design-system.md) | Fluent UI React v9、Wails 桌面壳与本机操作规范 |
| [Plugin Management Surface](./plugin-management-surface.md) | 宿主工作区与插件页面之间的边界，以及对所有插件页面的无障碍要求 |
| [静态资源来源与用途](./assets.md) | 图片、图标与字体的来源、运行引用和离线检查边界 |

## 采用矩阵

| 界面 | 状态 | 当前实现边界 | 采用完成条件 |
| --- | --- | --- | --- |
| 设计上下文 | `documented` | `PRODUCT.md`、`DESIGN.md` 与 `.impeccable/design.json` 提供战略、视觉和扩展元数据 | loader 能同时读取产品与设计上下文 |
| Web 管理面 | `adopted` | 管理壳、认证入口和全部正式工作区已映射项目级语义；界面为白色页面上以阴影分层的浅灰盒子，主题支持 `system`、`light`、`dark`；只提供桌面布局，以 16:9 的 `1920×1080` 为最低分辨率 | 后续变更沿用语义 token、状态色调、异常优先披露和桌面布局，并满足 Web 界面验收条件 |
| Launcher | `adopted` | Fluent theme、CSS variables、原生窗口背景与五个工作区已采用项目级语义；侧栏、内容分组与浮层是白色画布上的浅灰盒子，主操作为蓝色；界面在 `1920×1080` 屏幕上以默认 `1280×720` 窗口验证，最小窗口为 `960×560`，各尺寸均使用左侧导航 | 后续变更沿用语义 token、主题同步、桌面壳职责，并满足 Launcher 界面验收条件 |
| 官方插件 | `independent` | 各插件仓库维护自己的设计记录与样式，官方游戏插件参照对应游戏的官方视觉语言；产物自带组件运行时和样式 | 满足插件页面边界与无障碍要求即可，不要求采用宿主视觉语义 |
| 第三方插件 | `compatible-envelope` | 宿主负责 iframe 边界、载入状态、安全确认和错误恢复；页面不继承宿主全局样式 | 页面支持系统主题媒体查询、键盘操作、对比度和 reduced-motion，不要求使用 RayleaBot 组件 |
| 宿主主题同步 | `adopted` | Vue SDK 读取同源宿主页面的亮暗模式与主题变量并映射到插件页面根节点，宿主切换主题时同步更新；插件页面可采用、映射或忽略这些变量 | 主题变量集合变化时同步宿主、Vue SDK 和文档 |

`documented` 表示正式文档与机器可读上下文完整，`adopted` 表示运行时已采用并由对应界面测试约束，`independent` 表示由插件仓库维护自己的设计体系，`compatible-envelope` 表示插件可独立实现页面，不要求使用 RayleaBot 组件。

## 共同边界

- Web 使用 Reka UI、自有 shadcn-vue 组件、Tailwind CSS 与 Motion for Vue。
- Launcher 使用 Fluent UI React v9 与 Motion，并通过生成的 Wails bindings 连接 Go 桌面宿主。
- 插件管理页使用包内静态 HTML、CSS 和 JavaScript 产物，不依赖宿主组件运行时，视觉体系由插件仓库自行维护；官方插件的这些产物由 Vue 3、TypeScript 和 Vite 构建。
- Web 与 Launcher 共享颜色角色、字体层级、间距、圆角和状态语义，不共享框架组件；插件页面只共享无障碍门槛。
- 聊天图片模板属于渲染产物，不受本产品界面规范约束。

## 设计工具

Impeccable 的项目上下文由根目录 `PRODUCT.md` 和 `DESIGN.md` 提供。Web 与 Launcher 各有独立的 pnpm 工作区；从仓库根设置 `IMPECCABLE_CONTEXT_DIR`，使指定应用目标的命令读取共享上下文：

```powershell
$env:IMPECCABLE_CONTEXT_DIR = (Get-Location).Path
& '.agents/skills/impeccable/scripts/impeccable.cmd' context --target web/src
```

共享设计记录的维护在仓库根执行 `.agents/skills/impeccable/scripts/impeccable.cmd doctor --json`。设计稿、评审截图、交互会话和构建草稿在本地使用；`PRODUCT.md`、`DESIGN.md`、`design/tokens.json` 与 `.impeccable/design.json` 随仓库维护。
