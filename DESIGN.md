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

界面服务于配置、诊断、恢复和长期运行。紧凑标题、自然内容高度与稳定对齐让真实状态和下一步操作保持清楚；青瓷集中于品牌、主操作和少数选中标记，普通文字、边框、搜索框与选中背景保持中性。Web 与 Launcher 共享视觉语义，分别使用 Ant Design Vue 与 Fluent UI。

**Key Characteristics:**

- 灰白与炭灰画布、中性导航和连续工作区。
- 少量青瓷主操作、选中标记和几何折叶标识。
- 自托管 Noto Sans SC 用于 Web 普通文字，Launcher 正文使用系统字体；字号、字重与间距形成克制层级。
- 亮暗主题、键盘操作和窄屏呈现保持等价操作能力。
- 管理工作区的状态、表单、列表与日志采用不透明表面；玻璃用于浮层，认证入口采用独立的 Liquid Glass 浏览器适配。

本文件的 token 前置数据由 [design/tokens.json](design/tokens.json) 生成。它是机器值的唯一来源，采用 base → semantic light/dark → component 结构；[生成脚本](scripts/generate-design-tokens.mjs) 同时维护 Web、Launcher、favicon、共享字体 CSS 与 [.impeccable/design.json](.impeccable/design.json)。运行 `node scripts/generate-design-tokens.mjs` 更新生成物，运行 `node scripts/generate-design-tokens.mjs --check` 校验漂移、指定对比度与颜色边界。原生图标由独立的 [图标生成脚本](scripts/generate-launcher-icons.mjs) 维护。

前置数据、共享字体 CSS 与 sidecar 均由生成器维护，不直接编辑；sidecar 的 narrative 同步本文件正文。应用局部映射在正文与分面规范中说明，不改变共享基础 token 的含义。

## Colors

青瓷是低饱和的交互强调色；近白画布、实色表面和柔和结构边界构成主体内容。暗色主题使用炭灰背景与浅灰文字，青瓷只在对应品牌和操作角色中提高亮度。

### Primary

- **青瓷主操作**：浅色使用 light-primary，暗色使用 dark-primary；悬停与按下分别消费对应组件映射。
- **青瓷品牌前景**：折叶标识、品牌链接和少数勾选标记消费品牌角色，不扩散到普通正文和容器边界。

### Neutral

- **雾白画布与瓷白表面**：应用底色、表单和有独立任务边界的面板保持连续、轻盈的层次。
- **炭灰表面**：暗色画布、内容与导航。
- **中性选择与焦点**：导航选中背景、菜单选中背景与文字、搜索框、普通悬停面及键盘焦点使用中性角色；Web 导航当前项使用主题色文字，名为 brand-soft 的兼容 token 也表示中性淡面。
- **正文、辅文与边界**：结构边界负责分组；控件边界负责可操作目标，两者不能互换。

### Named Rules

**The Semantic Token Rule.** 运行代码只消费语义和组件 token；基础色阶仅用于建立映射。

**The Neutral Ground Rule.** 普通表面、边框、文字、搜索框和选中背景保持中性；青瓷只强调品牌、主操作和少数选中标记。

**The Semantic Independence Rule.** 人工关注、成功、警告和危险保持独立，状态同时提供文字、图标或结构化标签。

人工关注用于确认、可信代码提示和未保存草稿；警告表达降级，危险表达失败、阻塞或破坏性操作。恢复兼容、降级、阻塞分别使用 success、warning、danger，文字与图形使用同一语义。

## Typography

**Display Font:** 自托管 Noto Sans SC，回退为 Microsoft YaHei UI 与 sans-serif，用于品牌文字、页面标题和面板标题。共享 [typography.generated.css](design/typography.generated.css) 引入仓库已有 WOFF2 子集，两端随构建打包；[字体授权](templates/help.menu/assets/fonts/noto-sans-sc/OFL.txt) 随两端公开资源附带。

**Body Font:** 共享基础 token 保留 Segoe UI Variable Text、Segoe UI 与中文系统无衬线回退栈，Launcher 正文和标准控件使用该栈。Web 在 [`_base.scss`](web/src/styles/_base.scss) 的 `:root` 中将 `--font-sans` 局部映射到现有 `--font-display`，管理面与认证入口的 Ant Design theme 同时消费 `var(--font-sans)`；因此 Web 普通正文、控件与插件卡片版本使用自托管 Noto Sans SC。该映射不修改共享基础 token，也不影响 Launcher 或独立 iframe 的字体。

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

桌面 Web 使用持久导航（244px）与紧凑页头（60px）；面包屑、搜索、主题、全屏和偏好入口直接可达。Launcher 默认窗口为 1280×720，最小为 760×560，按可用工作区与最小尺寸约束调整；窗口包含原生标题栏（44px）、带文字导航（184px）与单一主工作区。具体分面规则见 [Web](docs/design/web-management-ui.md) 与 [Launcher](docs/design/launcher-design-system.md)。

间距使用前置 token 的 xs 至 xxl 标尺。独立任务可以使用完整有边界表面，同一任务内的字段、日志和数据行通过间距与分隔线组织。页面主操作位于稳定位置，状态总览保持连续横条，列表按真实内容排列。

插件集合使用一至五列的独立对象卡片网格，同排卡片等高、操作栏底部对齐，长元数据与健康提示允许内容自然增高。约 10–15 张完整卡片是 2K 桌面上的密度目标，实际数量随视口高度、内容和宽度偏好变化；不通过裁掉状态或缩小触控目标保证固定数量。断点与卡片入口见 [Web 分面规范](docs/design/web-management-ui.md)。

窄屏通过抽屉、换行与单列流调整结构；手机保留必要纵向滚动。技术表、长路径与日志可以在自身区域横向查看，普通页面不依赖整页横向滚动。正文和关键控件不随视口任意缩小，窄屏或粗指针交互目标使用至少 44px。

## Elevation & Depth

实色表面、精确边界与留白提供主要层级。管理工作区的表单、列表、日志和常规内容保持不透明。菜单、选择器浮层、抽屉和 Dialog 可使用静态玻璃：表面色占 90%，背景模糊固定为 12px；不随指针、滚动或动画改变模糊半径。浮层阴影表达覆盖关系，两套主题的阴影与 sticky、menu、drawer、modal、toast、emergency 层级由 sidecar 记录。

认证入口参考 Apple 的 [Liquid Glass 材质](https://developer.apple.com/videos/play/wwdc2025/219/)，在静态壁纸上使用通透面板、圆角透镜折射与边缘高光。支持 SVG backdrop 的 Chromium 路径使用几何法线图驱动折射，仅附加 1.2px 模糊；WebKit 与 Gecko 使用固定 5px 模糊、112% 饱和度的透明材质降级。这是浏览器适配，具体效果遵循浏览器能力。颜色通过现有认证主题 token 的 CSS `color-mix()` 局部派生；浅色面板表面色占 9%、高光占 18%，暗色分别为 18% 与 12%，浅色辅文与底部链接局部加深以保持对比度，不改变共享品牌 token。

不支持 backdrop-filter，或启用 reduced-transparency、forced-colors 时，浮层与认证面板使用完整不透明表面。内容可读性与操作反馈不依赖玻璃效果。

### Named Rules

**The Structural Shadow Rule.** 管理工作区的静态边框表面不叠加大阴影，阴影只说明真实浮层关系；认证入口的阴影用于表达玻璃面板与控件的材质层次。

**The Overlay Glass Rule.** 管理工作区的玻璃只属于覆盖内容的浮层，表单、列表和日志使用不透明底色；认证入口按专用材质规则呈现。所有玻璃表面始终保留不透明降级。

## Shapes

标准控件使用温和圆角（8px），独立任务表面与浮层采用较宽圆角（12px）；紧凑组件使用小圆角（4px / 6px），状态胶囊使用 full。认证面板圆角为 36px，窄屏为 28px，凭据输入和主按钮为 16px；这些局部圆角不作为通用控件标准。

品牌标识为四个色面组成的几何折叶，形状以 [design/mark.json](design/mark.json) 为唯一母版。Web、Launcher 与 favicon 共享该几何；色面随品牌角色或单色环境映射。Launcher 功能图标使用 Fluent Regular，品牌标识不承担操作或状态含义。

原生资产位于 [launcher/assets/](launcher/assets/)，应用 PNG、托盘 PNG 和 Windows ICO 由母版确定性生成，PNG 内嵌来源元数据，不属于 AI 生成图像。ICO 包含 16、24、32、48、64、128、256px 图像。运行 `node scripts/generate-launcher-icons.mjs --check` 校验来源与资产摘要；Windows 构建资源的验证入口见 Launcher 分面规范。

## Components

### Buttons

主按钮只强调当前工作流的主要动作，使用青瓷填充、对应前景与标准控件圆角。次级操作使用中性边界或轻量背景，人工关注和危险操作各用独立语义。默认、悬停、焦点、按下、禁用和加载状态均保留明确反馈。桌面标准高度为 36px，窄屏或粗指针目标为 44px。Launcher 在管理面可用时以打开管理面为主操作，否则使用现有启动动作；停止保持危险次级操作及原有确认规则。

### Inputs / Fields

常规业务字段采用实色表面，认证字段使用所属面板的局部玻璃材质；两者均保留完整控件边界和持续可见标签。错误说明关联字段，禁用状态保持可读，占位文本不承担标签职责。常规字段焦点使用专用 token，轮廓为 2px，间距为 2px；认证字段使用 1px 边框与紧贴边缘的 3px 柔和着色光环，不叠加分离的外轮廓。forced-colors 下均使用 2px 系统焦点轮廓。

### Navigation

当前项使用完整中性选择背景、高对比文字与功能图标，不依赖细侧边条或品牌标识表达当前位置。插件中心在侧栏使用一个顶级入口，菜单中心、插件商店、插件列表、插件设置和指令中心在内容区顶部切换，并共享一个可关闭工作区页签；原路由与页面缓存独立，插件详情保留自己的页签。其余分类保持稳定；移动导航以抽屉或紧凑形式提供同一能力。

插件中心五页以当前切换项承担可见页名，页面一级标题仅供辅助技术读取；无重复页头介绍。保存、刷新等操作与对应字段或筛选工具栏相邻，保留必要的状态和字段说明。

### Chips / Status

状态标签同时呈现文字或图标，不能只显示色点。标签表达状态和筛选，不替代操作按钮。关注提示、异常和空态提供原因、影响、可执行动作或必要前置条件。首页没有恢复摘要时显示“暂无恢复记录”，不能推断兼容通过；日志无匹配结果时说明为空，并提供调整筛选或等待新日志的方向。

### Cards / Containers

容器使用项目级表面与结构边界，正文按自然高度排列。字段组不层层包成卡片，日志只保留一个外框，其内部使用行分隔与独立正文滚动区。

插件卡片展示包内图标、名称及其后的版本、描述、安装来源类型、信任、运行状态和健康提示；图标缺失或加载失败时使用共享折叶 Logo。ID、作者与安装根目录保留在概要或详情。卡片底部提供概要、详情、重载和启停四个图标入口，每个入口具有可访问名称与提示；指令与别名在既有概要、详情中查看，不外显为卡片内容。该集合是有任务意义的卡片布局，不要求其他数据页采用卡片。

### Authentication

登录、首次初始化与凭据恢复指引共享最大宽度 448px 的居中单栏面板，保留折叶品牌、Noto Sans SC 与 Ant Design Vue。静态青瓷玻璃壁纸使用 [celadon-glass.png](web/src/assets/auth/celadon-glass.png)，图像内嵌生成提示词作为来源记录，按容器高度 140% 缩放并底部对齐。背景和面板不随指针移动；鼠标仅改变边缘高光位置，reduced-motion 下保持静态。离屏 Canvas 只在面板尺寸或圆角变化时生成几何法线图，空闲时没有持续绘制循环。

凭据输入和主按钮在桌面与窄屏均为 50px 高。面板仅在进入认证布局时执行 opacity / transform 动画（420ms、最多 8px 垂直位移），切换恢复指引不重复播放；低高度视口允许自然滚动，reduced-motion 下即时呈现，forced-colors 隐藏壁纸并使用系统表面与边界。认证区域文字选区使用现有品牌填充与对应前景。

“忘记密钥？”在登录面板内打开本机重置指引，返回时保留已填凭据并将焦点归还入口；重置由 Launcher 或停服后的 CLI 完成。入口、步骤与字段反馈见 [Web 认证规范](docs/design/web-management-ui.md#认证入口)。

### Motion and ownership

控件反馈采用 100–160ms 的短节奏，工作区采用 180–220ms，浮层采用 200–220ms；当前共有反馈、Web 内容切换、Launcher 工作区和浮层分别使用 160ms、200ms、220ms 和 220ms。动画只改变 opacity / transform 或控件状态属性，服务于选择、层级切换和显隐，持续日志不逐条播放进入动画。

Launcher 工作区从可见透明度（0.88）进入，状态与内容在点击时更新；连续切换取消旧动画并从当前可见程度继续。导航持续可交互，reduced-motion 或 forced-colors 下立即完成。Web 与 Launcher 均保留 system、light、dark 主题偏好，手动选择可持久化，不使用按时钟自动切换配置。动效时长不代表帧率或性能承诺。

**The Single Motion Owner Rule.** 同一元素只接受一种动效机制，连续操作取消旧动画并以最新状态为准。

**The Fold Mark Rule.** 折叶由同一几何母版生成，品牌不代替状态图标或导航文字。

插件页面、聊天卡片与渲染模板拥有独立内容和样式边界；其管理面 Host 使用本体系。独立 iframe 不继承宿主 CSS、字体或组件运行时，详见 [插件管理面](docs/design/plugin-management-surface.md)。

## Do's and Don'ts

### Do:

- Do 用中性导航、连续工作区和精确分隔线建立稳定方位。
- Do 将青瓷留给品牌、主操作和少数选中标记，保持普通表面与文字中性。
- Do 保持亮暗主题的信息层级、状态含义与操作能力等价。
- Do 使用项目 token、共享标题字体与标准框架控件。
- Do 保持可见焦点、键盘操作、触控目标、reduced-motion 与 forced-colors 支持。
- Do 让真实数据、必要警告和用户任务决定内容高度与页面密度。

### Don't:

- Don't 使用科技蓝、霓虹边界或通用深色科技仪表盘作为品牌语言。
- Don't 使用巨型标题、装饰性眉题、hero 指标模板或虚构数据填满页面。
- Don't 使用无任务意义的同尺寸卡片拼贴、多层日志外框或无任务边界的嵌套卡片；独立插件集合的卡片网格属于明确保留的例外。
- Don't 将认证入口的玻璃材质扩展到管理工作区的表单、列表或日志，不动画化模糊半径，也不逐条动画日志。
- Don't 依赖颜色单独表达状态，或用 attention 混同 warning 与 danger。
- Don't 为视觉风格引入平行组件库、运行时主题服务或跨 iframe 样式注入。
