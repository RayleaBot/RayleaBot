# Design

本目录承载 RayleaBot 产品界面的视觉、交互与分面映射规范。根目录 [`PRODUCT.md`](../../PRODUCT.md) 定义产品语境，根目录 [`DESIGN.md`](../../DESIGN.md) 定义项目级视觉规范，本目录只说明各产品界面如何在冻结技术栈中应用这些语义。

HTTP、WebSocket、schema、错误码、插件协议和发布元数据继续由 [`contracts/`](../../contracts/README.md) 裁决。设计文档不新增接口字段、状态名或客户端状态源。

## 阅读顺序

1. [`PRODUCT.md`](../../PRODUCT.md)：用户、产品目的、品牌性格、反例与无障碍目标。
2. [`DESIGN.md`](../../DESIGN.md)：颜色、字体、层次、组件与强制视觉规则。
3. 本目录分面文档：Web、Launcher 和插件管理页的组件体系映射。
4. 当前实现：各分面在采用矩阵中的状态与验收入口。

## 分面文档

| 文档 | 作用 |
| --- | --- |
| [Web Management UI](./web-management-ui.md) | Ant Design Vue、Vue Vben Admin 壳层与页面组合规范 |
| [Launcher Design System](./launcher-design-system.md) | Fluent UI React v9、Electron 桌面壳与本机操作规范 |
| [Plugin Management Surface](./plugin-management-surface.md) | 官方插件页面完整规范与第三方页面兼容包络 |

## 采用矩阵

| 分面 | 状态 | 当前实现边界 | 采用完成条件 |
| --- | --- | --- | --- |
| 设计上下文 | `documented` | `PRODUCT.md`、`DESIGN.md` 与 `.impeccable/design.json` 提供战略、视觉和扩展元数据 | loader 能同时读取产品与设计上下文，设计文件由 CI 识别为 docs |
| Web 管理面 | `pending-runtime` | 认证入口已采用项目级语义；管理壳仍使用现有 Ant Design Vue tokens，主题偏好只包含显式亮色和暗色 | 全局语义 token 完成映射，首次主题跟随系统，页面组合满足 Web 分面验收条件 |
| Launcher | `adopted` | Fluent theme、CSS variables、原生窗口背景与五个工作区已采用项目级冷暖语义；`760×560` 最小窗口使用顶部紧凑导航 | 后续变更保持语义 token、主题同步、桌面壳职责与 Launcher 分面验收条件 |
| 内置及官方插件 | `pending-runtime` | 页面各自维护本地 CSS，不共享运行时组件或全局样式 | 本地 token 映射符合项目规范，亮暗主题、焦点、状态和窄屏行为通过验收 |
| 第三方插件 | `compatible-envelope` | 宿主负责 iframe 边界、载入状态、安全确认和错误恢复；页面不获得宿主全局样式 | 页面支持系统主题媒体查询、键盘操作、对比度和 reduced-motion，不要求使用 RayleaBot 组件 |
| 宿主手动主题同步 | `requires-contract-first` | `host.init` 没有主题字段，页面不得依赖未声明消息 | 新字段先进入 bridge contract，再同步 fixture、宿主、官方插件示例和文档 |

`documented` 表示正式文档与机器可读上下文完整，`adopted` 表示运行时已采用并由对应分面测试约束，`pending-runtime` 表示运行时仍需采用，`compatible-envelope` 表示当前边界允许独立实现，`requires-contract-first` 表示实现前必须冻结正式契约。

## 共同边界

- Web 继续使用 Ant Design Vue 与 Vue Vben Admin 对齐方案。
- Launcher 继续使用 Fluent UI React v9 与现有 Electron 分层。
- 插件管理页继续使用包内 HTML、CSS 和 JavaScript，不获得宿主组件运行时。
- 三个分面共享颜色角色、字体层级、间距、圆角、状态语义和无障碍门槛，不共享框架组件。
- 聊天图片模板属于渲染产物，不受本产品界面规范约束。
