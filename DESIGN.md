---
name: RayleaBot
description: 雾白表面、青瓷绿与紧凑工作区组成的自托管机器人管理界面
colors:
  celadon-50: "#F1F6F2"
  celadon-100: "#E1E9E3"
  celadon-200: "#CEDCD1"
  celadon-300: "#BBD0C1"
  celadon-400: "#A3C5B3"
  celadon-500: "#80A48F"
  celadon-600: "#638673"
  celadon-700: "#476C5E"
  celadon-800: "#365749"
  celadon-900: "#294438"
  celadon-1000: "#18281F"
  light-canvas: "#FAFAFA"
  light-surface: "#FFFFFF"
  light-surface-raised: "#FFFFFF"
  light-text: "#252525"
  light-text-muted: "#666666"
  light-border: "#E3E3E3"
  light-control-border: "#8A8A8A"
  light-primary: "#476C5E"
  light-primary-hover: "#365749"
  light-primary-pressed: "#294438"
  light-on-brand: "#FFFFFF"
  light-brand-foreground: "#476C5E"
  light-brand-soft: "#F0F0F0"
  light-focus: "#555555"
  light-chrome: "#F7F7F7"
  light-nav-selected: "#E9E9E9"
  light-nav-selected-text: "#252525"
  light-attention: "#9B4A2F"
  light-attention-soft: "#F9ECE6"
  light-on-attention: "#FFFFFF"
  light-success: "#227653"
  light-success-soft: "#E8F4EE"
  light-warning: "#8A5600"
  light-danger: "#B9384E"
  dark-canvas: "#161616"
  dark-surface: "#1E1E1E"
  dark-surface-raised: "#292929"
  dark-text: "#EEEEEE"
  dark-text-muted: "#ADADAD"
  dark-border: "#3D3D3D"
  dark-control-border: "#858585"
  dark-primary: "#A3C5B3"
  dark-primary-hover: "#BBD0C1"
  dark-primary-pressed: "#80A48F"
  dark-on-brand: "#18281F"
  dark-brand-foreground: "#BBD0C1"
  dark-brand-soft: "#2B2B2B"
  dark-focus: "#C0C0C0"
  dark-chrome: "#1B1B1B"
  dark-nav-selected: "#333333"
  dark-nav-selected-text: "#EEEEEE"
  dark-attention: "#E08A61"
  dark-attention-soft: "#3B261E"
  dark-on-attention: "#18281F"
  dark-success: "#65D39A"
  dark-success-soft: "#193429"
  dark-warning: "#F0BB5A"
  dark-danger: "#FF8494"
typography:
  headline:
    fontFamily: "'Noto Sans SC', 'Microsoft YaHei UI', sans-serif"
    fontSize: "22px"
  title:
    fontFamily: "'Noto Sans SC', 'Microsoft YaHei UI', sans-serif"
    fontSize: "18px"
  section:
    fontFamily: "'Noto Sans SC', 'Microsoft YaHei UI', sans-serif"
    fontSize: "16px"
  body:
    fontFamily: "'Segoe UI Variable Text', 'Segoe UI', 'Microsoft YaHei UI', 'PingFang SC', 'Hiragino Sans GB', 'Noto Sans SC', system-ui, sans-serif"
    fontSize: "14px"
  label:
    fontFamily: "'Segoe UI Variable Text', 'Segoe UI', 'Microsoft YaHei UI', 'PingFang SC', 'Hiragino Sans GB', 'Noto Sans SC', system-ui, sans-serif"
    fontSize: "13px"
  mono:
    fontFamily: "'Cascadia Mono', Consolas, 'JetBrains Mono', 'Courier New', monospace"
    fontSize: "13px"
rounded:
  xs: "4px"
  sm: "6px"
  md: "8px"
  lg: "12px"
  full: "999px"
spacing:
  xs: "4px"
  sm: "8px"
  md: "12px"
  lg: "16px"
  xl: "24px"
  xxl: "32px"
components:
  button-primary:
    backgroundColor: "{colors.light-primary}"
    textColor: "{colors.light-on-brand}"
    rounded: "{rounded.md}"
    height: "36px"
  button-primary-hover:
    backgroundColor: "{colors.light-primary-hover}"
  button-primary-active:
    backgroundColor: "{colors.light-primary-pressed}"
  button-attention:
    backgroundColor: "{colors.light-attention}"
    textColor: "{colors.light-on-attention}"
    rounded: "{rounded.md}"
    height: "36px"
  input:
    backgroundColor: "{colors.light-surface-raised}"
    textColor: "{colors.light-text}"
    rounded: "{rounded.md}"
    height: "36px"
  navigation-item:
    backgroundColor: "{colors.light-nav-selected}"
    textColor: "{colors.light-nav-selected-text}"
    rounded: "{rounded.md}"
  status-chip:
    backgroundColor: "{colors.light-success-soft}"
    textColor: "{colors.light-success}"
    rounded: "{rounded.full}"
  section-surface:
    backgroundColor: "{colors.light-surface}"
    textColor: "{colors.light-text}"
    rounded: "{rounded.lg}"
  attention-callout:
    backgroundColor: "{colors.light-attention-soft}"
    textColor: "{colors.light-attention}"
    rounded: "{rounded.lg}"
  data-row:
    backgroundColor: "{colors.light-surface}"
    textColor: "{colors.light-text}"
---

# Design System: RayleaBot

## Overview

**Creative North Star: "雾白·青瓷"**

RayleaBot 使用中性灰白或炭灰表面、精确分隔线和少量青瓷强调组织管理任务。导航连接连续工作区，几何折叶标识提供轻盈的品牌识别；亮暗主题保持相同的信息层级、状态语义与操作能力。

界面服务于配置、诊断、恢复和长期运行。紧凑标题、自然内容高度与稳定对齐让真实状态和下一步操作保持清楚；青瓷集中于品牌、主操作和少数选中标记，普通文字、静态边框、搜索框与选中背景保持中性。Web 与 Launcher 共享视觉语义；Web 按工作区迁移至基于 Reka UI 的产品组件，Launcher 保持 Fluent UI。

Web 当前已落地 M1 产品组件基础、M2 协议中心与 M3 应用壳及认证入口。侧栏、工作区页签、搜索、偏好、主题菜单、登录和初始化表单已使用产品组件；M4 插件、治理、诊断等业务页面及 AppCard、AppStatusTag、RetryPanel 等公共业务组件仍待迁移，顶层 App 保留 Ant Design Vue provider 供这些页面使用。本文记录已落地范围，后续阶段按实际实现更新，迁移范围见 [Web UI 迁移说明](docs/design/web-ui-migration.md)。

**Key Characteristics:**

- 灰白与炭灰画布、中性导航和连续工作区。
- 少量青瓷主操作、选中标记和几何折叶标识。
- 自托管 Noto Sans SC 用于 Web 普通文字，Launcher 正文使用系统字体；字号、字重与间距形成克制层级。
- 亮暗主题、键盘操作和窄屏呈现保持等价操作能力。
- 管理工作区的状态、表单、列表与日志采用不透明表面；玻璃用于浮层，认证入口采用独立的 Liquid Glass 浏览器适配。

本文件的 token 前置数据由 [design/tokens.json](design/tokens.json) 生成。它是机器值的唯一来源，采用 base → semantic light/dark → component 结构；[生成脚本](scripts/generate-design-tokens.mjs) 同时维护 Web、Launcher、favicon、共享字体 CSS 与 [.impeccable/design.json](.impeccable/design.json)。运行 `node scripts/generate-design-tokens.mjs` 更新生成物，运行 `node scripts/generate-design-tokens.mjs --check` 校验漂移、指定对比度与颜色边界。原生图标由独立的 [图标生成脚本](scripts/generate-launcher-icons.mjs) 维护。

前置数据、共享字体 CSS 与 sidecar 均由生成器维护，不直接编辑；sidecar 的 narrative 同步本文件正文。sidecar 的通用组件预览表达共享基础，Web 产品组件的局部尺寸、焦点与浮层生命周期以本文对应规则和组件源码为准。应用局部映射在正文与分面规范中说明，不改变共享基础 token 的含义。

## Colors

青瓷是低饱和的交互强调色；近白画布、实色表面和柔和结构边界构成主体内容。暗色主题使用炭灰背景与浅灰文字，青瓷只在对应品牌和操作角色中提高亮度。

### Primary

- **青瓷主操作**：浅色使用 light-primary，暗色使用 dark-primary；悬停与按下分别消费对应组件映射。
- **青瓷品牌前景**：折叶标识、品牌链接和少数勾选标记消费品牌角色，不扩散到普通正文和容器边界。

### Neutral

- **雾白画布与瓷白表面**：应用底色、表单和有独立任务边界的面板保持连续、轻盈的层次。
- **炭灰表面**：暗色画布、内容与导航。
- **中性选择与焦点**：导航选中背景、菜单选中背景与文字、搜索框和普通悬停面使用中性角色；Web 导航当前项使用主题色文字，名为 brand-soft 的兼容 token 也表示中性淡面。共享基础焦点保持中性；Web 基础按钮、输入框与选择器在基础焦点轮廓之外增加主题强调色边框与光环，见 Inputs / Fields。
- **正文、辅文与边界**：结构边界负责分组；控件边界负责可操作目标，两者不能互换。

### Named Rules

**The Semantic Token Rule.** 运行代码只消费语义和组件 token；基础色阶仅用于建立映射。

**The Neutral Ground Rule.** 普通表面、静态边框、文字、搜索框和选中背景保持中性；青瓷用于品牌、主操作和少数选中标记，已迁移 Web 产品控件的键盘焦点遵循局部焦点规则。

**The Semantic Independence Rule.** 人工关注、成功、警告和危险保持独立，状态同时提供文字、图标或结构化标签。

人工关注用于确认、可信代码提示和未保存草稿；警告表达降级，危险表达失败、阻塞或破坏性操作。恢复兼容、降级、阻塞分别使用 success、warning、danger，文字与图形使用同一语义。

## Typography

**Display Font:** 自托管 Noto Sans SC，回退为 Microsoft YaHei UI 与 sans-serif，用于品牌文字、页面标题和面板标题。共享 [typography.generated.css](design/typography.generated.css) 引入仓库已有 WOFF2 子集，两端随构建打包；[字体授权](templates/help.menu/assets/fonts/noto-sans-sc/OFL.txt) 随两端公开资源附带。

**Body Font:** 共享基础 token 保留 Segoe UI Variable Text、Segoe UI 与中文系统无衬线回退栈，Launcher 正文和标准控件使用该栈。Web 在 [`_base.scss`](web/src/styles/_base.scss) 的 `:root` 中将 `--font-sans` 局部映射到现有 `--font-display`；旧业务页面的 Ant Design theme 消费 `var(--font-sans)`，已迁移的管理面与认证产品组件继承同一字体映射，因此 Web 普通正文、控件与插件卡片版本使用自托管 Noto Sans SC。认证主题通过 CSS 变量映射，不再设置专用 Ant Design provider。该映射不修改共享基础 token，也不影响 Launcher 或独立 iframe 的字体。

**Label/Mono Font:** 标签沿用所在应用的正文栈。Web 日志行的时间、来源与技术元数据，详情中的来源、插件 ID、请求 ID，以及结构化数据、JSON 和代码使用 Cascadia Mono、Consolas、JetBrains Mono 等宽回退栈；日志消息正文使用 Noto Sans SC，不因位于 `pre` 中而改用等宽字体。

### Hierarchy

- **Headline**：管理页面标题保持紧凑（20–22px），项目主标题 token 为 22px；认证面板标题使用局部字号（28px，窄屏 26px）。
- **Title / Section**：分区、面板与组标题（18px / 16px）。
- **Body**：正文与标准控件（14px）。
- **Label / Mono**：标签、表头与技术元数据（13px）。
- 辅助元数据使用最小一级字号（12px）；关键操作不用该级字号。少量真实主状态可以使用 token 中的 26px 级，不形成巨型指标区。
- 连续说明正文最大宽度为 72ch；表格、日志和技术工作区按内容需要延展。

### Named Rules

**The Quiet Hierarchy Rule.** 层级依靠字号、字重和间距建立，不通过装饰性眉题、渐变文字或巨型指标制造噪音。

## Layout

桌面 Web 使用持久导航（244px，收起后 64px）与紧凑页头（60px）；面包屑、搜索、主题、全屏和偏好入口直接可达。991px 及以下通过左侧导航抽屉提供相同入口，目标宽度为 280px；偏好从右侧抽屉打开，目标宽度为 380px。Launcher 默认窗口为 1280×720，最小为 760×560，按可用工作区与最小尺寸约束调整；窗口包含原生标题栏（44px）、带文字导航（184px）与单一主工作区。具体分面规则见 [Web](docs/design/web-management-ui.md) 与 [Launcher](docs/design/launcher-design-system.md)。

间距使用前置 token 的 xs 至 xxl 标尺。独立任务可以使用完整有边界表面，同一任务内的字段、日志和数据行通过间距与分隔线组织。页面主操作位于稳定位置，状态总览保持连续横条，列表按真实内容排列。

插件集合使用一至五列的独立对象卡片网格，同排卡片等高、操作栏底部对齐，长元数据与健康提示允许内容自然增高。约 10–15 张完整卡片是 2K 桌面上的密度目标，实际数量随视口高度、内容和宽度偏好变化；不通过裁掉状态或缩小触控目标保证固定数量。断点与卡片入口见 [Web 分面规范](docs/design/web-management-ui.md)。

协议中心也保留独立连接卡片与页头“添加连接”入口。网格按可用宽度自动排列，单卡目标最小宽度为 320px，窄于该值时占满容器；卡片间距为 20px，639px 及以下为 16px。协议选择位于居中的配置弹窗内；配置、兼容矩阵与确认弹窗分别采用 640px、1040px 与 440px 的目标宽度，并受视口边界约束。

窄屏通过抽屉、换行与单列流调整结构；手机保留必要纵向滚动。技术表、长路径与日志可以在自身区域横向查看，普通页面不依赖整页横向滚动。正文和关键控件不随视口任意缩小，窄屏或粗指针交互目标使用至少 44px。

## Elevation & Depth

实色表面、精确边界与留白提供主要层级。管理工作区的表单、列表、日志和常规内容保持不透明。菜单、选择器浮层、抽屉和 Dialog 可使用静态玻璃：表面色占 90%，背景模糊固定为 12px；不随指针、滚动或动画改变模糊半径。浮层阴影表达覆盖关系，两套主题的阴影与 sticky、menu、drawer、modal、toast、emergency 层级由 sidecar 记录。

已迁移 Web 产品弹窗、抽屉、菜单与选择器浮层使用不透明的 raised surface，覆盖内容的表面消费现有 floating 阴影，不增加玻璃或背景模糊。遮罩由现有语义色以 42% 占比和透明色混合，浅色使用正文色、暗色使用画布色。Tooltip 使用正文色作底、表面色作文字，保持独立的高对比提示。

**The Web Overlay Stack Rule.** Web 弹窗与抽屉遮罩从 1200 起按打开顺序递增 20，内容位于所属遮罩上方 1 层；嵌套菜单和选择器继承所属层级再加 5，Tooltip 加 8。未嵌套菜单、选择器和 Tooltip 的基准为 1100，Toast 为 1600，使退出中的菜单留在新打开的抽屉下方。这些 Web 局部层级不改变共享基础层级或 Launcher。

认证入口参考 Apple 的 [Liquid Glass 材质](https://developer.apple.com/videos/play/wwdc2025/219/)，在静态壁纸上使用通透面板、圆角透镜折射与边缘高光。支持 SVG backdrop 的 Chromium 路径使用几何法线图驱动折射，仅附加 1.2px 模糊；WebKit 与 Gecko 使用固定 5px 模糊、112% 饱和度的透明材质降级。这是浏览器适配，具体效果遵循浏览器能力。颜色通过现有认证主题 token 的 CSS `color-mix()` 局部派生；浅色面板表面色占 9%、高光占 18%，暗色分别为 18% 与 12%，浅色辅文与底部链接局部加深以保持对比度，不改变共享品牌 token。

不支持 backdrop-filter，或启用 reduced-transparency、forced-colors 时，浮层与认证面板使用完整不透明表面。内容可读性与操作反馈不依赖玻璃效果。

### Named Rules

**The Structural Shadow Rule.** 管理工作区的静态边框表面不叠加大阴影，阴影只说明真实浮层关系；认证入口的阴影用于表达玻璃面板与控件的材质层次。

**The Overlay Glass Rule.** 管理工作区的玻璃只属于覆盖内容的浮层，表单、列表和日志使用不透明底色；认证入口按专用材质规则呈现。所有玻璃表面始终保留不透明降级。

## Shapes

标准控件使用温和圆角（8px），独立任务表面与浮层采用较宽圆角（12px）；紧凑组件使用小圆角（4px / 6px），状态胶囊使用 full。认证面板圆角为 36px，窄屏为 28px，凭据输入和主按钮为 16px；这些局部圆角不作为通用控件标准。

Web 产品按钮、输入框与选择器使用现有 lg 圆角（12px），居中产品弹窗采用局部圆角（桌面 16px，639px 及以下 14px）。配置、兼容矩阵、搜索与确认弹窗共享该形状，内部字段通过间距与分隔线分组。左右抽屉贴齐视口边缘、使用直角；菜单与 Toast 使用 12px 圆角，Tooltip 使用现有 md 圆角（8px）。

品牌标识为四个色面组成的几何折叶，形状以 [design/mark.json](design/mark.json) 为唯一母版。Web、Launcher 与 favicon 共享该几何；色面随品牌角色或单色环境映射。Launcher 功能图标使用 Fluent Regular，品牌标识不承担操作或状态含义。

原生资产位于 [launcher/assets/](launcher/assets/)，应用 PNG、托盘 PNG 和 Windows ICO 由母版确定性生成，PNG 内嵌来源元数据，不属于 AI 生成图像。ICO 包含 16、24、32、48、64、128、256px 图像。运行 `node scripts/generate-launcher-icons.mjs --check` 校验来源与资产摘要；Windows 构建资源的验证入口见 Launcher 分面规范。

## Components

### Buttons

主按钮只强调当前工作流的主要动作，使用青瓷填充、对应前景与标准控件圆角。次级操作使用中性边界或轻量背景，人工关注和危险操作各用独立语义。默认、悬停、焦点、按下、禁用和加载状态均保留明确反馈。共享基础桌面高度为 36px，窄屏或粗指针目标为 44px。Launcher 在管理面可用时以打开管理面为主操作，否则使用现有启动动作；停止保持危险次级操作及原有确认规则。

已迁移 Web 使用 [`AppButton`](web/src/components/AppButton.vue)：默认样式为中性描边，主要动作显式使用青瓷填充；危险动作使用淡危险底色与危险文字。默认和图标按钮均为 40px 高，粗指针下最小高度为 44px。加载状态保留动作文字，同时禁用重复提交并显示忙碌语义；减少动态效果或强制颜色时保留静态反馈。

### Inputs / Fields

常规业务字段采用实色表面，认证字段使用所属面板的局部玻璃材质；两者均保留完整控件边界和持续可见标签。错误说明关联字段，禁用状态保持可读，占位文本不承担标签职责。共享基础字段焦点使用专用 token，轮廓为 2px，间距为 2px；认证字段使用 1px 边框与紧贴边缘的 3px 柔和着色光环，不叠加分离的外轮廓。forced-colors 下使用系统焦点轮廓。

已迁移 Web 使用 [`AppField`](web/src/components/AppField.vue) 关联持续可见标签、控件和错误说明，字段内部间距为 8px，字段尾部间距为 24px；标签为 14px，说明为 13px。[`AppInput`](web/src/components/AppInput.vue) 与 [`AppSelect`](web/src/components/AppSelect.vue) 默认高度为 40px，粗指针下至少 44px；多选文字可换行并自然增高。密码显示开关保留可访问名称和按下状态，错误通过文字与 `aria-invalid` 一起表达；支持清空的输入框在清空后保留输入焦点。认证字段使用 Authentication 中的局部尺寸与材质。

**The Web Product Focus Rule.** Web 基础按钮、输入框与选择器通过 Tailwind 映射消费现有语义 token；键盘焦点保留基础 2px 轮廓，并使用主题强调色边框与 3px、50% 透明度的光环，错误状态使用独立危险语义。导航、页签、分段选择与菜单使用各自的中性焦点或高亮轮廓；认证字段遵循专用样式。局部规则不替换 Launcher 或旧 Web 页面的焦点样式，强制颜色模式保留系统可见焦点。

### Navigation

当前项使用完整中性选择背景、高对比文字与功能图标，不依赖细侧边条或品牌标识表达当前位置。Web 侧栏使用原生导航按钮，分类保留展开与收起；插件中心从顶级入口进入持久侧栏层，按管理、工具与已安装插件分组展示固定页面和插件资源。返回主导航只改变导航层级，保留当前页面；收起侧栏通过菜单提供固定入口与已打开插件，移动抽屉沿用展开侧栏的层级。

**The Sidebar Resource Rule.** 插件资源的打开按钮与展开按钮为同一行中的兄弟控件，分别负责导航与展开管理页，具有独立的可访问名称和焦点；不嵌套可交互按钮。插件可独立展开概览和 manifest 声明的管理页，展开状态、加载反馈与失败重试围绕对应资源呈现。

插件中心五页共享一个可关闭工作区页签，保留各自路由、缓存和最后访问地址；内容区不重复展示插件中心切换栏，插件详情保留独立页签和页面标题。工作区页签使用 Reka 手动激活，激活项由青瓷文字、底部标记与中性淡面共同表达；关闭按钮与页签触发器为兄弟控件，右键菜单提供既有关闭操作。横向空间不足时页签在自身区域滚动，保存、刷新等业务操作仍与对应字段或工具栏相邻。

### Tabs and segmented controls

[`AppTabs`](web/src/components/AppTabs.vue) 用于抽屉内等局部分区，采用 Reka 自动激活与 44px 高的标签目标；选中项使用青瓷文字和 2px 底线，标签列表可横向滚动。[`AppSegmented`](web/src/components/AppSegmented.vue) 用于主题、密度、页面切换和内容宽度等单选偏好：中性底座承载等宽选项，选中项使用实色表面、中性边界和轻阴影，默认目标高 36px，粗指针下至少 44px。两者保留禁用和可见键盘焦点；工作区页签的手动激活规则不套用于局部偏好标签。

### Menus and transient feedback

[`AppDropdown`](web/src/components/AppDropdown.vue) 统一按钮菜单与右键菜单，最小宽度为 180px、最大为 360px，并保留至少 8px 的视口碰撞余量；超长菜单在自身区域滚动。菜单项采用中性高亮，危险动作使用独立危险语义，粗指针目标至少 44px。主题菜单在 system、light、dark 之间选择，并以文字和勾选标记共同表达当前偏好。

[`AppTooltip`](web/src/components/AppTooltip.vue) 在 450ms 延迟后提供简短补充说明，宽度随内容展开，上限为 20rem 或视口宽度减 24px 中的较小值；普通短标签不压成单字竖排。提示不替代触发器的可访问名称。搜索使用目标宽度为 640px 的居中弹窗，打开后聚焦输入框，结果展示页面名称与路径，支持上下选择、Enter 导航和 Escape 关闭。

[`AppToastHost`](web/src/components/AppToastHost.vue) 在右上方显示最多四条即时反馈，通知宽度不超过 380px，并保留窄屏边距。每条提示包含语义图标、可换行正文和手动关闭入口；普通提示停留 4.5 秒，错误提示为 7 秒，关闭后保留 160ms 退场。持续问题留在页面状态中，不依赖短暂 Toast 承载。[`AppSpinner`](web/src/components/AppSpinner.vue) 提供状态文字或辅助技术可读名称，[`AppSkeleton`](web/src/components/AppSkeleton.vue) 提供忙碌语义；reduced-motion 或 forced-colors 下停止旋转、脉冲和提示过渡。

### Chips / Status

状态标签同时呈现文字或图标，不能只显示色点。标签表达状态和筛选，不替代操作按钮。关注提示、异常和空态提供原因、影响、可执行动作或必要前置条件。首页没有恢复摘要时显示“暂无恢复记录”，不能推断兼容通过；日志无匹配结果时说明为空，并提供调整筛选或等待新日志的方向。

### Cards / Containers

容器使用项目级表面与结构边界，正文按自然高度排列。字段组不层层包成卡片，日志只保留一个外框，其内部使用行分隔与独立正文滚动区。

插件卡片展示包内图标、名称及其后的版本、描述、安装来源类型、信任、运行状态和健康提示；图标缺失或加载失败时使用共享折叶 Logo。ID、作者与安装根目录保留在概要或详情。卡片底部提供概要、详情、重载和启停四个图标入口，每个入口具有可访问名称与提示；指令与别名在既有概要、详情中查看，不外显为卡片内容。该集合是有任务意义的卡片布局，不要求其他数据页采用卡片。

协议连接卡片展示名称、文字状态、运行摘要和连接标识，配置与删除入口固定在底部操作区。卡片最小高度为 220px，内边距为 20px，长名称、摘要与身份信息允许换行增高。已保存配置与当前运行状态分别说明，需要重启的变更在列表上方提示；卡片不将保存成功推断为连接已运行。

### Web product dialogs and progressive fields

Web 产品组件基于 Vue 3、Reka UI 2.10.4、仓库持有的 shadcn-vue / reka-nova 源码、Tailwind CSS 4 与 motion-v 2.4.2，工程版本以 [package.json](web/package.json) 和 [Web 工程基线](docs/engineering/web-admin-baseline.md) 为准。业务页面复用 App 组件层；底层交互语义由 Reka UI 承担，主题映射集中在 [tailwind.css](web/src/styles/tailwind.css)，浮层动效集中在 [presets.ts](web/src/motion/presets.ts)。

协议中心的“添加连接”先在配置弹窗内展示协议名称与说明，选择后在同一弹窗填写配置；“更换协议”返回选择步骤。常用地址、凭据、接收消息和启用状态优先展示，连接标识、沙箱、共用重连策略、令牌兼容选项与运行诊断按适用条件逐级展开。高级字段出错时展开对应区域并聚焦错误控件。保存和取消留在固定页脚，正文独立滚动；共享重连参数明确说明影响所有连接。

[`AppDialog`](web/src/components/AppDialog.vue) 的居中模式以 CSS 固定定位保持视口中心，桌面左右至少留 16px、上下至少留 24px，639px 及以下四周至少留 12px。标题、说明与关闭按钮留在页头，操作留在页脚；正文长度变化时按实测内容高度过渡，达到视口上限后只滚动正文。兼容矩阵沿用同一弹窗，其表格在自身区域横向滚动并固定能力名称列，手机显示横向滚动提示。

[`AppDrawer`](web/src/components/AppDrawer.vue) 复用 AppDialog 的左侧或右侧呈现，固定高度为 100dvh，宽度不超过视口减 24px；正文填充余下高度并独立滚动，页头与页脚保持可达。抽屉不插值内容高度，居中弹窗继续由 Motion 管理实测高度，避免两种呈现互相覆盖定位和动画样式。

**The Web Dialog Lifecycle Rule.** Web 产品弹窗与抽屉在关闭动画完成前保留 Reka 内容、遮罩、焦点约束与原有业务内容；由 Motion 的 animationComplete 完成退场后再释放层级、触发 afterClose 并归还焦点，调用方不得随 open 变为 false 提前卸载内容。原触发项已卸载时归还到明确的备用入口，例如从菜单打开设置后返回仍有效的菜单按钮。嵌套确认和选择器服从所属层级；确认弹窗使用 alertdialog，并将初始焦点放在取消操作。忙碌状态阻止关闭与重复提交，未保存修改通过确认弹窗处理。

### Authentication

登录、首次初始化与凭据恢复指引共享最大宽度 448px 的居中单栏面板，保留折叶品牌与 Noto Sans SC，凭据表单使用 AppField、AppInput、AppButton 和 AppAlert。静态青瓷玻璃壁纸使用 [celadon-glass.png](web/src/assets/auth/celadon-glass.png)，图像内嵌生成提示词作为来源记录，按容器高度 140% 缩放并底部对齐。背景和面板不随指针移动；鼠标仅改变边缘高光位置，reduced-motion 下保持静态。离屏 Canvas 只在面板尺寸或圆角变化时生成几何法线图，空闲时没有持续绘制循环。

凭据输入和主按钮在桌面与窄屏均为 50px 高。面板仅在进入认证布局时执行 opacity / transform 动画（420ms、最多 8px 垂直位移），切换恢复指引不重复播放；低高度视口允许自然滚动，reduced-motion 下即时呈现，forced-colors 隐藏壁纸并使用系统表面与边界。认证区域文字选区使用现有品牌填充与对应前景。

[`AuthCredentialsForm`](web/src/components/auth/AuthCredentialsForm.vue) 的标签、输入与错误保持关联，账号和密码使用相同控件高度与局部焦点样式。密码可见性按钮为 44px，具有可访问名称和按下状态；提交期间输入、显示开关和提交按钮均禁用，字段校验失败时聚焦首个错误输入。认证专用 CSS 变量由 [preferences/auth.ts](web/src/preferences/auth.ts) 映射，玻璃材质保持在 AuthLayout 内。

“忘记密钥？”在登录面板内打开本机重置指引，返回时保留已填凭据并将焦点归还入口；重置由 Launcher 或停服后的 CLI 完成。入口、步骤与字段反馈见 [Web 认证规范](docs/design/web-management-ui.md#认证入口)。

### Motion and ownership

控件反馈采用 100–160ms 的短节奏，工作区采用 180–220ms，浮层采用 200–220ms；当前共有反馈、Web 内容切换、Launcher 工作区和浮层分别使用 160ms、200ms、220ms 和 220ms。动画主要改变 opacity / transform 或控件状态属性，服务于选择、层级切换和显隐，持续日志不逐条播放进入动画。Web 居中弹窗另对实测内容高度进行过渡，以保持展开字段和异步内容变化时的居中关系。

Web 产品弹窗与抽屉入场为 220ms、退场为 160ms，沿用现有缓动 `cubic-bezier(0.16, 1, 0.3, 1)`。居中模式改变透明度与缩放（0.96 ↔ 1），CSS 定位负责居中，Motion 负责显隐和实测高度；抽屉保持缩放为 1，以透明度和最多 24px 的水平位移表现打开方向。reduced-motion 或 forced-colors 下时长为零、缩放为 1、位移为零，同时保留完整关闭生命周期与焦点归还；forced-colors 下保留系统表面边界。

Web 工作区优先使用只捕获主内容的 View Transition，缺少能力时由统一的 Motion for Vue 入口降级，侧栏和页头保持可交互。菜单通过自身 CSS 状态过渡显隐（160ms），Toast 沿用同组过渡（进入 200ms、退出 160ms）；这些元素不叠加第二套 Motion 动画，减少动态效果或强制颜色时即时呈现。

Launcher 工作区从可见透明度（0.88）进入，状态与内容在点击时更新；连续切换取消旧动画并从当前可见程度继续。导航持续可交互，reduced-motion 或 forced-colors 下立即完成。Web 与 Launcher 均保留 system、light、dark 主题偏好，手动选择可持久化，不使用按时钟自动切换配置。动效时长不代表帧率或性能承诺。

**The Single Motion Owner Rule.** 同一元素只接受一种动效机制，连续操作取消旧动画并以最新状态为准。

**The Fold Mark Rule.** 折叶由同一几何母版生成，品牌不代替状态图标或导航文字。

插件页面、聊天卡片与渲染模板拥有独立内容和样式边界；其管理面 Host 使用本体系。独立 iframe 不继承宿主 CSS、字体或组件运行时，详见 [插件管理面](docs/design/plugin-management-surface.md)。

## Do's and Don'ts

### Do:

- Do 用中性导航、连续工作区和精确分隔线建立稳定方位。
- Do 将青瓷留给品牌、主操作和少数选中标记，保持普通表面与文字中性。
- Do 保持亮暗主题的信息层级、状态含义与操作能力等价。
- Do 使用项目 token、共享标题字体与所属应用的标准产品组件；Web 按已落地工作区遵守迁移边界。
- Do 保持可见焦点、键盘操作、触控目标、reduced-motion 与 forced-colors 支持。
- Do 让真实数据、必要警告和用户任务决定内容高度与页面密度。

### Don't:

- Don't 使用科技蓝、霓虹边界或通用深色科技仪表盘作为品牌语言。
- Don't 使用巨型标题、装饰性眉题、hero 指标模板或虚构数据填满页面。
- Don't 使用无任务意义的同尺寸卡片拼贴、多层日志外框或无任务边界的嵌套卡片；独立插件集合与协议连接卡片属于明确保留的对象布局。
- Don't 将认证入口的玻璃材质扩展到管理工作区的表单、列表或日志，不动画化模糊半径，也不逐条动画日志。
- Don't 依赖颜色单独表达状态，或用 attention 混同 warning 与 danger。
- Don't 在已迁移 Web 工作区新增另一套控件或浮层行为；迁移期间保留旧工作区所需依赖，不以此扩展新的平行组件体系。
- Don't 为视觉风格引入运行时主题服务或跨 iframe 样式注入。
