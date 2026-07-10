# RayleaBot Launcher Design System

本规范服务于 `launcher/` 的 Electron 桌面启动器。项目级视觉语义以根目录 [`DESIGN.md`](../../DESIGN.md) 为准；Launcher 继续使用 React 18、Fluent UI React v9 和现有 main、preload、renderer 分层。

## 产品职责

- Launcher 是本机服务壳、环境预检入口和 Web 管理面入口，不复制 Web 的业务页面或服务端状态机。
- 界面优先回答服务是否可用、是否需要人工处理、当前可以执行什么操作。
- 主操作保持唯一突出；停止、重置和退出等危险操作使用危险语义；工具操作保持低权重。
- 系统诊断、恢复和运行环境状态直接展示正式服务结果，不从日志文本推断状态。

## 主题与 token 映射

Launcher 支持 `system`、`light` 和 `dark`，首次显示跟随系统。`FluentProvider` 与自定义 CSS variables 必须使用同一有效主题，窗口背景、原生控件和自定义表面保持一致。

| 产品语义 | Fluent / Launcher 角色 | 浅色 token | 暗色 token |
| --- | --- | --- | --- |
| 窗口画布 | 页面与窗口背景 | `light-canvas` | `dark-canvas` |
| 内容表面 | Fluent surface 与独立面板 | `light-surface` | `dark-surface` |
| 抬升表面 | Dialog、Popover、浮动确认 | `light-surface` | `dark-surface-raised` |
| 主文本 | `colorNeutralForeground1` | `light-text` | `dark-text` |
| 次文本 | `colorNeutralForeground2` | `light-text-muted` | `dark-text-muted` |
| 边界 | `colorNeutralStroke1` | `light-border` | `dark-border` |
| 主操作与选择 | 品牌色、焦点、当前导航 | `light-cool-action` | `cool-signature` |
| 人工关注 | 本地 attention token | `light-warm-attention` | `warm-signature` |
| 状态 | Fluent semantic colors | 浅色语义 tokens | 暗色语义 tokens |

人工关注色只标记需要操作者判断或确认的事项，不替代 warning、danger 或普通 primary action。

## 桌面壳结构

- 顶部拖动区负责窗口标题与原生窗口控制，不承载页面主操作。
- 宽窗口使用 `72px` 紧凑导航栏和单一主内容区；导航项由图标、可访问名称和选中淡面组成。
- 状态、运行环境、设置和关于信息保持稳定分区，切换时保留当前任务上下文。
- 主内容区优先使用单列任务流；只有状态与操作真实并行时才使用双列。
- 日志与路径使用等宽字体，保持可选择、可复制和可横向查看。

## Fluent 组件映射

| 场景 | 组件 | 规则 |
| --- | --- | --- |
| 主操作 | `Button appearance="primary"` | 当前工作流保持唯一，使用冷色主操作语义 |
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
- 独立面板使用 `12px` 圆角和低对比阴影，Dialog 使用 `16px` 圆角和浮层阴影。
- 普通说明与字段不包裹为卡片；状态摘要不使用 hero 指标模板。
- 选中导航使用完整淡面和轮廓，不使用内嵌彩色侧边条。

## 动效与反馈

- 控件状态使用 `160ms`，分区切换使用 `200ms`，Dialog 使用 `220ms`。
- 动效统一使用 `cubic-bezier(0.16, 1, 0.3, 1)`，只表达状态、反馈、载入和内容显隐。
- 悬停不平移或缩放面板；按下状态通过色面和边界变化表达。
- `prefers-reduced-motion` 下关闭非必要动画，进度与忙碌状态保留静态文字或图标。

## 窄窗口策略

- 窗口宽度低于 `900px` 时，垂直导航转为顶部或横向紧凑导航，主内容保持单列。
- 低于 `640px` 时，操作按优先级换行，路径和日志允许横向滚动，不截断关键标识符。
- 窄窗口仍保留启动、停止、预检、设置、恢复入口和打开 Web 管理面的完整能力。

## 验收条件

- `system`、`light` 和 `dark` 三种偏好均同步作用于 `FluentProvider`、CSS variables 和窗口背景。
- 主操作、人工关注、警告和危险具有独立且稳定的语义。
- 界面不存在嵌套卡片、巨型指标、彩色侧边条、玻璃材质或装饰性动效。
- 键盘顺序、焦点、对比度、状态标签和 reduced-motion 达到 WCAG 2.2 AA。
- Launcher 继续只负责本机进程、系统集成、更新确认和打开 Web，不复制管理面业务。
