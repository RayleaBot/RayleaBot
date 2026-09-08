# RayleaBot Launcher Design System

本规范服务于 `launcher/` 的 Wails 桌面启动器。项目级视觉语义以根目录 [`DESIGN.md`](../../DESIGN.md) 为准；Launcher 使用 React 19、Fluent UI React v9、Go 桌面宿主和生成的 typed bindings。

## 产品职责

- Launcher 是本机服务壳、环境预检入口和 Web 管理面入口，不复制 Web 的业务页面或服务端状态机。
- 界面优先回答服务是否可用、是否需要人工处理、当前可以执行什么操作。
- 主操作保持唯一突出：正式可用性允许打开 Web 时显示“管理界面”，否则显示现有启动动作。停止保持危险次级操作及现有确认规则；重置与退出沿用既有危险语义，工具操作保持低权重。
- 系统诊断、恢复和运行环境状态直接展示正式服务结果，不从日志文本推断状态。

## 主题与 token 映射

Launcher 支持 `system`、`light` 和 `dark`，首次显示跟随系统，显式选择可持久化，不提供按时钟自动切换配置。`FluentProvider` 与自定义 CSS variables 必须使用同一有效主题，窗口背景、原生控件和自定义表面保持一致。普通画布、文字、边框和选中背景使用灰白或炭灰中性色，青瓷用于品牌、主操作和少数选中标记。`design/tokens.json` 是唯一机器值源，`launcher/src/shared/launcher-theme-tokens.generated.ts` 提供生成值，`launcher-theme.ts` 保留既有消费接口。

主题入口使用显式菜单，按“跟随系统、浅色、深色”排列并显示当前单选项。菜单由 Fluent Motion 在 `220ms` 内淡入或淡出并伴随最多 `5px` 的垂直位移；选中反馈在退出期间保持可见，弹层消失后主题通过 `200ms` 根快照交叉淡化，并把焦点还给触发按钮。`prefers-reduced-motion` 下立即完成开合与主题切换。

| 产品语义 | Fluent / Launcher 角色 | 浅色 token | 暗色 token |
| --- | --- | --- | --- |
| 窗口画布 | 页面与窗口背景 | `light-canvas` | `dark-canvas` |
| 内容表面 | Fluent surface 与独立面板 | `light-surface` | `dark-surface` |
| 抬升表面 | Dialog、Popover、浮动确认 | `light-surface-raised` | `dark-surface-raised` |
| 主文本 | `colorNeutralForeground1` | `light-text` | `dark-text` |
| 次文本 | `colorNeutralForeground2` | `light-text-muted` | `dark-text-muted` |
| 边界 | `colorNeutralStroke1` | `light-border` | `dark-border` |
| 主操作填充 | `colorBrandBackground`、主按钮 | `light-primary` | `dark-primary` |
| 品牌前景 | 品牌链接、标识和少量选中标记 | `light-brand-foreground` | `dark-brand-foreground` |
| 焦点 | `colorStrokeFocus2` 与全局焦点轮廓 | `light-focus` | `dark-focus` |
| 导航表面 | 标题栏与 184px 导航栏 | `light-chrome` | `dark-chrome` |
| 普通选中项 | 导航、菜单背景与文字 | `light-nav-selected`、`light-nav-selected-text` | `dark-nav-selected`、`dark-nav-selected-text` |
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
| `setup_required` | running | 按运行中展示；管理界面负责首次初始化流程 |
| `failed` | danger | 服务启动或就绪失败 |

## 桌面壳结构

- 默认窗口为 `1280×720` 逻辑像素，最小窗口为 `760×560`；创建窗口时根据屏幕工作区与最小尺寸约束调整大小。
- 顶部拖动区高度为 `44px`，与导航栏共同使用中性表面；顶部放置折叶标识、窗口标题与原生窗口控制，不放置页面主操作。
- 宽窗口使用 `184px` 带文字导航栏和单一主内容区；导航项由 Fluent Regular 功能图标、可见文字、可访问名称和完整中性选中色面组成。
- 运行状态、环境检查、日志诊断、偏好设置和关于应用保持稳定分区，切换时保留当前任务上下文。
- 主内容区优先使用单列任务流；只有状态与操作真实并行时才使用双列。
- 日志与路径使用等宽字体，保持可选择、可复制和可横向查看。
- 文本选区使用浏览器和操作系统的默认高亮，不通过 `::selection` 覆盖选区背景或文字颜色；控件选中态独立使用组件主题。
- 主要任务区使用完整中性结构边界，次级信息通过数据行、分隔线和留白组织；表单、列表和日志保持不透明，阴影仅表达浮层关系。
- 环境检查始终展示阻塞与警告项，正常项按系统核心、运行环境和环境特性分组收起。
- 尚未取得检查结果时显示“尚未检查”；发生本地错误且无检查结果时显示“检查结果不可用”及重新检查入口，不给出可以启动的结论。
- 偏好设置的阅读态使用定义行；输入框、路径选择和单选控件只在编辑态出现。
- 日志诊断使用单一纵向滚动正文区，技术快照默认收起，实际异常日志保持优先可见。
- 服务状态、运行详情和无异常摘要按紧凑数据行排列；主状态与操作保持同一工作区，不用装饰性指标填充留白。

## Fluent 组件映射

| 场景 | 组件 | 规则 |
| --- | --- | --- |
| 主操作 | `Button appearance="primary"` | 当前工作流保持唯一，使用青瓷主操作语义 |
| 人工确认 | `Button` + attention token | 只用于需要明确判断的动作，不与警告色混用 |
| 危险操作 | `Button` + danger token | 停止、重置和完全退出，必须有明确结果文案 |
| 次级操作 | `Button` | 使用中性边界和表面，不与主操作竞争 |
| 工具操作 | `Button appearance="subtle"` | 编辑路径、刷新、复制和导航工具 |
| 文本输入 | `Input` | 完整标签、清晰焦点和禁用状态 |
| 单项选择 | `RadioGroup`、`Radio` | 关闭策略和互斥设置 |
| 状态 | `Badge`、`MessageBar` | 同时提供文字、图标或结构化标签 |
| 确认 | `Dialog` | 仅用于破坏性或不可在原位安全完成的决定 |

自定义 CSS 只补充窗口布局、日志表面和 Fluent token 无法表达的最小业务差异。页面不得重新实现 Fluent 已提供的按钮、输入、单选、Badge 或 Dialog。

功能图标统一使用 Fluent Regular。折叶用于应用身份，功能图标用于导航、状态与操作，两者不互相替代。

## 密度与层次

- 桌面控件高度为 `36px`，导航与关键操作目标至少为 `40px`；窄窗口和触控场景提升到 `44px`。
- 页面间距使用项目级 `4/8/12/16/24/32px` 标尺，导航、字段和面板按任务关系选择不同节奏。
- 圆角标尺为 `4/6/8/12/999px`；标准控件使用 `8px`，任务表面和浮层使用 `12px`。
- 静态独立面板使用 `12px` 圆角与 `1px` 边界，不叠加大范围阴影；运行状态主任务区使用正式状态图标和语义边界，Dialog 使用浮层阴影。
- 普通说明与字段不包裹为卡片；状态摘要不使用 hero 指标模板。
- 选中导航使用完整背景、高对比文字和功能图标，不使用内嵌彩色侧边条。
- 页面标题保持在 `20–22px`，分区标题为 `18px`，组标题为 `16px`，正文为 `14px`，任务标签、路径和日志为 `13px`。`12px` 用于窗口 chrome 与辅助元数据；运行状态主值使用 `24px`、`600` 字重和 `1.35` 行高。
- 品牌、页面与面板标题使用共享自托管 Noto Sans SC，正文和 Fluent 控件沿用系统字体栈。字体子集及授权文件随 Launcher 构建打包。
- Vite 开发服务器仅对 Launcher、共享设计目录及共享字体目录开放文件访问；开发与生产均须实际加载声明的标题字体。

## 动效与反馈

- 控件反馈采用 `100–160ms` 短节奏，工作区采用 `180–220ms`，浮层采用 `200–220ms`；当前控件状态为 `160ms`，工作区切换与 Dialog 为 `220ms`，主题切换为 `200ms`。
- 控件、浮层和主题动效使用 `cubic-bezier(0.16, 1, 0.3, 1)`，只表达状态、反馈、载入和内容显隐。
- 工作区使用 `220ms` 可取消的 WAAPI 透明度动画，采用 `cubic-bezier(0.25, 0.1, 0.25, 1)`，从 `0.88` 的可见透明度进入；状态与页面内容在点击时立即替换，导航在动画期间持续可交互。主题使用 View Transition API，主题选择器使用 Fluent Motion presence。连续操作取消旧动画并从当前可见程度继续，不使用计时器推断完成状态。
- WAAPI 或 View Transition 不可用时立即替换工作区或主题，功能与焦点顺序保持完整。同一元素不叠加 View Transition、Fluent Motion、WAAPI 和 CSS 动画。
- 悬停不平移或缩放面板；按钮按下可以使用 `1px` 的短位移，状态仍由色面和边界表达。日志追加不逐条播放进入动画。
- `prefers-reduced-motion` 或 forced-colors 下关闭非必要动画，进度与忙碌状态保留静态文字或图标。动效时长不代表帧率或性能承诺。
- 减少动态效果分别作用于 Fluent 控件、主题浮层、工作区和进度反馈，不依赖全局极短动画时长覆盖；未知进度保留进行中的文字和无障碍状态。

## 浮层材质

- 主题菜单和 Dialog 使用 `90%` 表面色与固定 `12px` backdrop blur；普通内容、表单、列表和日志保持不透明。
- 选中背景和文字使用中性色，玻璃不改变状态或操作语义，也不参与动态模糊。
- 不支持 backdrop-filter，或启用 reduced-transparency、forced-colors 时，浮层降为完整不透明表面。

## 原生图标与打包

- [`design/mark.json`](../../design/mark.json) 是折叶几何母版；[`scripts/generate-launcher-icons.mjs`](../../scripts/generate-launcher-icons.mjs) 生成 [`launcher/assets/`](../../launcher/assets/) 中的 SVG 来源、应用 PNG、托盘 PNG 与 Windows ICO。
- 应用 PNG 为 `1024×1024`，托盘 PNG 为 `32×32`；PNG 内嵌确定性来源元数据。资产由已有几何生成，不属于 AI 生成图像。
- Windows ICO 包含 `16/24/32/48/64/128/256px` 七种尺寸。Go 宿主消费应用与托盘 PNG，Windows EXE 通过图标资源消费 ICO，使应用、窗口、任务栏与托盘使用同一品牌母版。
- Windows 打包使用冻结的 Wails `v3.0.0-beta.9`，在原生 Go build 前由 `build-package.mjs` 生成对应架构的 `rsrc_windows_<arch>.syso`；Windows manifest 使用 `asInvoker` 普通用户权限。
- 在仓库根目录运行 `node scripts/generate-launcher-icons.mjs --check` 校验来源与资产摘要。在 Windows 上运行 `python launcher/scripts/verify-windows-icon-resources.py`，验证默认打包 EXE 中的七尺寸图像负载与源 ICO 逐字节一致；其他产物通过 `--exe <path>` 指定。资源校验与实际窗口、任务栏、托盘显示检查分别承担不同验证职责。

## 窄窗口策略

- Launcher 最小窗口为 `760×560`，所有工作区、Dialog 与窗口控制在该尺寸下保持可操作。
- 窗口宽度低于 `900px` 时，垂直导航转为顶部或横向紧凑导航，主内容保持单列。
- `760–899px` 范围内，关键操作目标提升到 `44px`，标题操作、表单和日志按优先级换行或滚动，不截断关键标识符。
- 窄窗口导航与主题工具区保持单行，位于完整的 `44px` 标题栏之下；刷新操作放在对应工作区页头。
- 宽度低于 `900px` 且高度不超过 `650px` 时，导航高度保持在 `52–56px`，工作区压缩外层留白和区域间距，不缩小正文与关键操作目标。
- Renderer 根节点不依赖隐藏溢出裁切内容；横向滚动只出现在日志、技术快照或不可安全换行的标识符区域。
- 窄窗口仍保留启动、停止、预检、设置、恢复入口和打开 Web 管理面的完整能力。

## 验收条件

- `system`、`light` 和 `dark` 三种偏好均同步作用于 `FluentProvider`、CSS variables 和窗口背景。
- 主操作、人工关注、警告和危险具有独立且稳定的语义。
- 界面不存在嵌套卡片、巨型指标、彩色侧边条或装饰性循环动效；玻璃仅用于具有不透明降级的主题菜单和 Dialog。
- 键盘顺序、焦点、对比度、状态标签、reduced-motion 和 forced-colors 达到 WCAG 2.2 AA。
- Launcher 继续只负责本机进程、系统集成、更新确认和打开 Web，不复制管理面业务。
