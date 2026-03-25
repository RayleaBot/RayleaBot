# RayleaBot Launcher Design System

本设计系统服务于 `launcher/` 的 Windows 桌面启动器界面，目标是建立克制、清晰、可维护的深色 Fluent 工具风格。

## 设计原则

- 启动器是本地服务壳和 Web 入口，不做大屏展示面板。
- 视觉层级以信息效率优先，避免重复 hero、重边框和深色表面反复嵌套。
- 主操作必须唯一突出；危险操作明确标识；工具操作统一降级。
- 组件样式通过 tokens 和通用 patterns 复用，不在页面中散落硬编码颜色与间距。

## Tokens

### 颜色

| Token | 用途 |
| --- | --- |
| `Color.Background` | 窗口主背景 |
| `Color.NavigationPane` | 左侧导航背景 |
| `Color.Surface` | 默认内容卡片 |
| `Color.SurfaceElevated` | 页头、工具栏、二级信息表面 |
| `Color.SurfaceRaised` | badge、次级按钮背景 |
| `Color.SurfaceMuted` | 日志面板、空状态、只读文本区 |
| `Color.BorderSubtle` | 低对比 1px 边框 |
| `Color.BorderStrong` | 更强但仍克制的边界强调 |
| `Color.TextPrimary` | 主文字 |
| `Color.TextSecondary` | 次文字 |
| `Color.TextMuted` | 标签、辅助信息 |
| `Color.Accent` | 主操作强调色 |
| `Color.Success` / `Color.Warning` / `Color.Error` | 语义色 |
| `Color.SuccessSurface` / `Color.WarningSurface` / `Color.ErrorSurface` | 语义面板底色 |

### 尺寸

| Token | 值 | 用途 |
| --- | --- | --- |
| `Spacing.Page` | `20` | 页面区块节奏 |
| `Spacing.Section` | `16` | 卡片之间的默认间距 |
| `Spacing.Row` | `12` | 表单行、定义列表行距 |
| `Spacing.Inline` | `8` | 行内按钮和 badge 间距 |
| `Thickness.PagePadding` | `20` | 页面内容外边距 |
| `Thickness.CardPadding` | `16` | 主卡片内边距 |
| `Thickness.CompactCardPadding` | `12` | 工具栏、提示条、次级面板内边距 |
| `Size.ControlHeight` | `38` | 主按钮、标准输入框高度 |
| `Size.CompactControlHeight` | `32` | 工具按钮高度 |
| `Radius.Small` / `Medium` / `Large` | `8 / 10 / 12` | 统一圆角 |
| `Size.NavPaneOpen` / `Compact` | `220 / 52` | 导航展开与收起宽度 |

### 字体

| Token | 用途 |
| --- | --- |
| `FontFamily.Ui` | 常规界面文本 |
| `FontFamily.Display` | 页面标题和导航主标题 |
| `FontFamily.Mono` | 日志与诊断文本 |
| `Font.PageTitle` | 页面标题 |
| `Font.SectionTitle` | 分区标题 |
| `Font.Body` | 正文 |
| `Font.Caption` | 标签、说明、摘要 |

## 组件语义

### Page Header

- `page-header` 用于环境检查、日志与诊断、设置页。
- 内容固定为：caption、title、summary。
- 不承担主状态展示，不重复展示窗口级产品标题。

### Hero Header

- `hero-header` 仅用于状态页。
- 内容固定为：标题、状态 badge、一句说明。
- 高度受控，不承载二级摘要和操作区。

### App Card

- `app-card` 为页面主要承载容器。
- `sub-card` 为卡内次级信息块，层级低于 `app-card`。
- 页面内最多使用两层表面深度，避免“深色底再套一层深色底”。

### Toolbar Card

- `toolbar-card` 承载工具按钮与统计 badge。
- 用于日志页工具栏、环境页摘要工具区、设置页编辑状态栏。

### Issue Banner

- `issue-banner` 用于首页的单条问题提示。
- 采用内联、低高度、单行主信息 + 次说明 + 单个 CTA 的结构。
- 不替代完整问题列表。

### Metric Tile

- `metric-tile` 用于状态页环境摘要等紧凑指标卡。
- 只展示 1 个标签和 1 个主值，不叠加复杂说明。

### Check Item

- `check-item` 用于环境检查页。
- 建议结构：
  - 标题
  - 严重等级 badge
  - 一句话问题描述
  - 证据
  - 处理建议
  - 操作按钮

### Log Panel

- `log-panel` 与 `log-textbox` 用于最近错误和诊断原文。
- 统一使用等宽字体、低反差背景、适合复制的内边距。

### Empty State

- `empty-state` 用于无错误输出、无检查项等场景。
- 只提供简短说明，不使用插画或装饰。

## 按钮层级

- `PrimaryActionButton`
  - 当前页面唯一主操作。
  - 在状态页中优先用于“启动服务”或“打开管理界面”。
- `DangerActionButton`
  - 用于停止服务等危险操作。
  - 强调风险，但不与主按钮竞争页面主视觉。
- `SecondaryActionButton`
  - 用于编辑路径、浏览文件等明确但非主路径操作。
- `ToolbarActionButton`
  - 用于重试、复制诊断、打开日志目录等工具栏操作。
- `SubtleActionButton`
  - 用于低频、低权重辅助动作。

## 页面约束

### 状态页

- 首屏必须同时容纳：hero、问题提示条、服务信息、环境摘要、主操作。
- 最近错误输出位于主内容区内，不应把整页拉成长滚动面板。

### 环境检查页

- 页面目标是帮助用户优先处理阻塞项和警告项。
- 正常项默认折叠，安装与打包信息降级为次级信息。

### 日志与诊断页

- 优先展示工具栏、最近错误、结构化诊断摘要。
- 原始诊断文本保留，但不作为唯一摘要形式。

### 设置页

- 默认只读。
- 进入编辑态后才允许修改路径和布尔设置。
- 保存按钮仅在存在未保存修改时可用。

## 可维护性规则

- 新页面优先组合现有 tokens、`LauncherControls.axaml` 和 `LauncherPatterns.axaml`，而不是复制旧页面样式。
- 新增状态颜色、间距和控件高度前，优先复用既有 tokens。
- 页面内如需特殊样式，应先评估是否能抽为通用 pattern，再决定是否局部定义。
