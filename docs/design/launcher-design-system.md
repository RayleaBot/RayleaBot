# RayleaBot Launcher Design System

本规范服务于 `launcher/` 的 Electron 桌面启动器。项目级视觉语义以根目录 [`DESIGN.md`](../../DESIGN.md) 为准；Launcher 继续使用 React 18、Fluent UI React v9 和现有 main、preload、renderer 分层。

## 产品职责

- Launcher 是本机服务壳、环境预检入口和 Web 管理面入口，不复制 Web 的业务页面或服务端状态机。
- 界面优先回答服务是否可用、是否需要人工处理、当前可以执行什么操作。
- 主操作保持唯一突出；停止、重置和退出等危险操作使用危险语义；工具操作保持低权重。
- 系统诊断、恢复和运行环境状态直接展示正式服务结果，不从日志文本推断状态。

## 主题与 token 映射

Launcher 支持 `system`、`light` 和 `dark`，首次显示跟随系统。`FluentProvider` 与自定义 CSS variables 必须使用同一有效主题，窗口背景、原生控件和自定义表面保持一致。`design/tokens.json` 是唯一机器值源，`launcher/src/shared/launcher-theme-tokens.generated.ts` 提供生成值，`launcher-theme.ts` 保留既有消费接口。

主题入口使用显式菜单，按“跟随系统、浅色、深色”排列并显示当前单选项。菜单由 Fluent Motion 在 `220ms` 内淡入或淡出并伴随最多 `5px` 的垂直位移；选中反馈在退出期间保持可见，弹层消失后主题通过 `200ms` 根快照交叉淡化，并把焦点还给触发按钮。`prefers-reduced-motion` 下立即完成开合与主题切换。

| 产品语义 | Fluent / Launcher 角色 | 浅色 token | 暗色 token |
| --- | --- | --- | --- |
| 窗口画布 | 页面与窗口背景 | `light-canvas` | `dark-canvas` |
| 内容表面 | Fluent surface 与独立面板 | `light-surface` | `dark-surface` |
| 抬升表面 | Dialog、Popover、浮动确认 | `light-surface` | `dark-surface-raised` |
| 主文本 | `colorNeutralForeground1` | `light-text` | `dark-text` |
| 次文本 | `colorNeutralForeground2` | `light-text-muted` | `dark-text-muted` |
| 边界 | `colorNeutralStroke1` | `light-border` | `dark-border` |
| 主操作填充 | `colorBrandBackground`、主按钮 | `light-primary` | `dark-primary` |
| 品牌前景 | 链接、标识和强调文字 | `light-brand-foreground` | `dark-brand-foreground` |
| 焦点 | `colorStrokeFocus2` 与全局焦点轮廓 | `light-focus` | `dark-focus` |
| 品牌壳层 | 标题栏与 76px 导航轨 | `light-chrome` | `dark-chrome` |
| 品牌填充内容 | `colorNeutralForegroundOnBrand` | `on-brand` | `on-brand` |
| 人工关注 | 本地 attention token | `light-attention` | `dark-attention` |
| 状态 | Fluent semantic colors | 浅色语义 tokens | 暗色语义 tokens |

人工关注色只标记需要操作者判断或确认的事项，不替代 warning、danger 或普通 primary action。

服务状态使用固定色调映射，页标题与工作区共用同一语义：

| 服务状态 | 色调 | 含义 |
| --- | --- | --- |
| `stopped` | neutral | 服务处于普通停止状态 |
| `starting`、`stopping` | info | 启动器正在执行系统操作 |
| `running` | success | 服务可用 |
| `degraded` | warning | 服务可用但能力受限 |
| `setup_required` | attention | 需要操作者完成初始化 |
| `failed` | danger | 服务启动或就绪失败 |

## 桌面壳结构

- 顶部拖动区和 76px 导航轨共同使用深梅紫品牌壳层；顶部只承载定位器标识、窗口标题与原生窗口控制，不承载页面主操作。
- 宽窗口使用 `76px` 紧凑导航轨和单一主内容区；导航项由图标、可访问名称、完整选中色面和定位器标记组成。
- 运行状态、环境检查、日志诊断、偏好设置和关于应用保持稳定分区，切换时保留当前任务上下文。
- 主内容区优先使用单列任务流；只有状态与操作真实并行时才使用双列。
- 日志与路径使用等宽字体，保持可选择、可复制和可横向查看。
- 每个工作区最多使用一个带阴影的主要任务面，次级信息通过数据行、分隔线和留白组织。
- 环境检查始终展示阻塞与警告项，正常项按系统核心、运行环境和环境特性分组收起。
- 偏好设置的阅读态使用定义行；输入框、路径选择和单选控件只在编辑态出现。
- 日志诊断使用单一纵向滚动正文区，技术快照默认收起，实际异常日志保持优先可见。

## Fluent 组件映射

| 场景 | 组件 | 规则 |
| --- | --- | --- |
| 主操作 | `Button appearance="primary"` | 当前工作流保持唯一，使用梅紫主操作语义 |
| 人工确认 | `Button` + attention token | 只用于需要明确判断的动作，不与警告色混用 |
| 危险操作 | `Button` + danger token | 停止、重置和完全退出，必须有明确结果文案 |
| 次级操作 | `Button` | 使用中性边界和表面，不与主操作竞争 |
| 工具操作 | `Button appearance="subtle"` | 编辑路径、刷新、复制和导航工具 |
| 文本输入 | `Input` | 完整标签、清晰焦点和禁用状态 |
| 单项选择 | `RadioGroup`、`Radio` | 关闭策略和互斥设置 |
| 状态 | `Badge`、`MessageBar` | 同时提供文字、图标或结构化标签 |
| 确认 | `Dialog` | 仅用于破坏性或不可在原位安全完成的决定 |

自定义 CSS 只补充窗口布局、日志表面和 Fluent token 无法表达的最小业务差异。页面不得重新实现 Fluent 已提供的按钮、输入、单选、Badge 或 Dialog。

## 密度与层次

- 桌面控件高度为 `36px`，导航与关键操作目标至少为 `40px`；窄窗口和触控场景提升到 `44px`。
- 页面间距使用项目级 `4/8/12/16/24/32px` 标尺，导航、字段和面板按任务关系选择不同节奏。
- 圆角限定为 `4/6/10/14/999px`；标准控件使用 `10px`，任务表面和浮层使用 `14px`。
- 静态独立面板使用 `14px` 圆角与 `1px` 边界，不叠加大范围阴影；运行状态主任务面使用定位器母题和语义边界，Dialog 使用 `14px` 圆角和浮层阴影。
- 普通说明与字段不包裹为卡片；状态摘要不使用 hero 指标模板。
- 选中导航使用完整背景、高对比文字和定位器，不使用内嵌彩色侧边条。
- 运行状态主状态词作为全窗口唯一展示级元素使用 `28–30px`；页面标题为 `24px`，关键状态与分区标题为 `18px`，组标题为 `16px`，正文为 `14px`，任务标签、路径和日志为 `13px`。`12px` 只用于窗口 chrome 与短状态标签。

## 动效与反馈

- 控件状态使用 `160ms`，分区切换使用 `300ms`，Dialog 使用 `220ms`。
- 控件、浮层和主题动效使用 `cubic-bezier(0.16, 1, 0.3, 1)`，只表达状态、反馈、载入和内容显隐。
- 工作区使用 `300ms` 可取消的 WAAPI 透明度动画，采用 `cubic-bezier(0.25, 0.1, 0.25, 1)` 保持完整时长可感知；状态与页面内容在点击时立即替换，导航在动画期间持续可交互。主题使用 View Transition API，主题选择器使用 Fluent Motion presence。连续操作取消旧动画，不使用计时器推断完成状态。
- WAAPI 或 View Transition 不可用时立即替换工作区或主题，功能与焦点顺序保持完整。同一元素不叠加 View Transition、Fluent Motion、WAAPI 和 CSS 动画。
- 悬停不平移或缩放面板；按下状态通过色面和边界变化表达。
- `prefers-reduced-motion` 下关闭非必要动画，进度与忙碌状态保留静态文字或图标。

## 窄窗口策略

- Launcher 最小窗口为 `760×560`，所有工作区、Dialog 与窗口控制在该尺寸下保持可操作。
- 窗口宽度低于 `900px` 时，垂直导航转为顶部或横向紧凑导航，主内容保持单列。
- `760–899px` 范围内，关键操作目标提升到 `44px`，标题操作、表单和日志按优先级换行或滚动，不截断关键标识符。
- 宽度低于 `900px` 且高度不超过 `650px` 时，导航高度保持在 `52–56px`，工作区压缩外层留白和区域间距，不缩小正文与关键操作目标。
- Renderer 根节点不依赖隐藏溢出裁切内容；横向滚动只出现在日志、技术快照或不可安全换行的标识符区域。
- 窄窗口仍保留启动、停止、预检、设置、恢复入口和打开 Web 管理面的完整能力。

## 验收条件

- `system`、`light` 和 `dark` 三种偏好均同步作用于 `FluentProvider`、CSS variables 和窗口背景。
- 主操作、人工关注、警告和危险具有独立且稳定的语义。
- 界面不存在嵌套卡片、巨型指标、彩色侧边条、玻璃材质或装饰性动效。
- 键盘顺序、焦点、对比度、状态标签、reduced-motion 和 forced-colors 达到 WCAG 2.2 AA。
- Launcher 继续只负责本机进程、系统集成、更新确认和打开 Web，不复制管理面业务。
