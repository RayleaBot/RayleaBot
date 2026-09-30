---
name: RayleaBot
description: 白色页面、浅灰卡片与柔和阴影组成的自托管机器人管理界面
colors:
  azure-50: "#EEF3FD"
  azure-100: "#DCE6FB"
  azure-200: "#BACEF6"
  azure-300: "#8DB3FF"
  azure-400: "#3567DC"
  azure-500: "#2860D8"
  azure-600: "#1F52C0"
  azure-700: "#1A47A8"
  azure-800: "#163B8A"
  azure-900: "#11306E"
  azure-1000: "#0A1B40"
  light-canvas: "#FFFFFF"
  light-surface: "#F5F5F3"
  light-surface-raised: "#FFFFFF"
  light-surface-soft: "#ECEBE8"
  light-text: "#1C1B19"
  light-text-muted: "#5F5C56"
  light-border: "#E4E2DE"
  light-control-border: "#8A8782"
  light-primary: "#2860D8"
  light-primary-hover: "#1F52C0"
  light-primary-pressed: "#1A47A8"
  light-on-brand: "#FFFFFF"
  light-brand-foreground: "#2860D8"
  light-brand-soft: "#E8EEFB"
  light-focus: "#2860D8"
  light-chrome: "#F5F5F3"
  light-nav-selected: "#FFFFFF"
  light-nav-selected-text: "#1C1B19"
  light-attention: "#6240B8"
  light-attention-soft: "#EEE8FB"
  light-on-attention: "#FFFFFF"
  light-success: "#1D6E3D"
  light-success-soft: "#DDF1E3"
  light-warning: "#7F5000"
  light-danger: "#A92E29"
  light-info: "#2159B8"
  light-info-soft: "#E0E9FB"
  dark-canvas: "#111110"
  dark-surface: "#1C1C1A"
  dark-surface-raised: "#282826"
  dark-surface-soft: "#161615"
  dark-text: "#EEEDEA"
  dark-text-muted: "#B4B1AA"
  dark-border: "#2F2E2B"
  dark-control-border: "#86837C"
  dark-primary: "#3567DC"
  dark-primary-hover: "#2B5BD0"
  dark-primary-pressed: "#2451BF"
  dark-on-brand: "#FFFFFF"
  dark-brand-foreground: "#8DB3FF"
  dark-brand-soft: "#1F2A44"
  dark-focus: "#8DB3FF"
  dark-chrome: "#1C1C1A"
  dark-nav-selected: "#2E2E2B"
  dark-nav-selected-text: "#EEEDEA"
  dark-attention: "#B9A5FF"
  dark-attention-soft: "#2A2342"
  dark-on-attention: "#1A1433"
  dark-success: "#5BD08B"
  dark-success-soft: "#173323"
  dark-warning: "#F3BB4F"
  dark-danger: "#FF8177"
  dark-info: "#88AEFF"
  dark-info-soft: "#1B2A45"
typography:
  headline:
    fontFamily: "'HarmonyOS Sans SC', 'Microsoft YaHei UI', 'PingFang SC', sans-serif"
    fontSize: "28px"
  title:
    fontFamily: "'HarmonyOS Sans SC', 'Microsoft YaHei UI', 'PingFang SC', sans-serif"
    fontSize: "18px"
  section:
    fontFamily: "'HarmonyOS Sans SC', 'Microsoft YaHei UI', 'PingFang SC', sans-serif"
    fontSize: "16px"
  body:
    fontFamily: "'HarmonyOS Sans SC', 'Microsoft YaHei UI', 'PingFang SC', 'Hiragino Sans GB', system-ui, sans-serif"
    fontSize: "14px"
  label:
    fontFamily: "'HarmonyOS Sans SC', 'Microsoft YaHei UI', 'PingFang SC', 'Hiragino Sans GB', system-ui, sans-serif"
    fontSize: "13px"
  mono:
    fontFamily: "'Cascadia Mono', Consolas, 'JetBrains Mono', 'Courier New', monospace"
    fontSize: "13px"
rounded:
  xs: "6px"
  sm: "8px"
  md: "12px"
  lg: "16px"
  xl: "20px"
  xxl: "24px"
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
    rounded: "{rounded.full}"
    height: "40px"
  button-primary-hover:
    backgroundColor: "{colors.light-primary-hover}"
  button-primary-active:
    backgroundColor: "{colors.light-primary-pressed}"
  button-attention:
    backgroundColor: "{colors.light-attention}"
    textColor: "{colors.light-on-attention}"
    rounded: "{rounded.full}"
    height: "40px"
  input:
    backgroundColor: "{colors.light-surface-raised}"
    textColor: "{colors.light-text}"
    rounded: "{rounded.md}"
    height: "40px"
  navigation-item:
    backgroundColor: "{colors.light-nav-selected}"
    textColor: "{colors.light-nav-selected-text}"
    rounded: "{rounded.full}"
  status-chip:
    backgroundColor: "{colors.light-success-soft}"
    textColor: "{colors.light-success}"
    rounded: "{rounded.full}"
  section-surface:
    backgroundColor: "{colors.light-surface}"
    textColor: "{colors.light-text}"
    rounded: "{rounded.xl}"
  raised-control:
    backgroundColor: "{colors.light-surface-raised}"
    textColor: "{colors.light-text}"
    rounded: "{rounded.full}"
    height: "40px"
  attention-callout:
    backgroundColor: "{colors.light-attention-soft}"
    textColor: "{colors.light-attention}"
    rounded: "{rounded.xl}"
  data-row:
    backgroundColor: "{colors.light-surface}"
    textColor: "{colors.light-text}"
---

# Design System: RayleaBot

## Overview

**Creative North Star: "白底浅灰"**

RayleaBot 使用白色页面、浅灰盒子与柔和阴影组织管理任务，蓝色只用于主操作、链接与焦点。导航连接连续工作区，黑白人物标识提供品牌识别；亮暗主题保持相同的信息层级、状态语义与操作能力。

界面服务于配置、诊断、恢复和长期运行。紧凑标题、自然内容高度与稳定对齐让真实状态和下一步操作保持清楚；盒子只以阴影与页面分开，页面上的控件为浅灰、盒子里的控件为白色，普通文字、分隔线与选中背景保持中性。Web 与 Launcher 共享视觉语义；Web 使用基于 Reka UI 的产品组件，Launcher 使用 Fluent UI。

Web 正式路由统一使用 Vue 3、Reka UI、自有 shadcn-vue 组件源码、Tailwind CSS 与 Motion for Vue，覆盖应用壳、认证、协议、插件、账号、治理、配置和诊断工作区。主题、字体与密度通过共享 CSS 变量和产品组件提供，页面复用统一的交互与反馈机制。对象卡片、分区表单、虚拟列表、非模态日志窗口与独立 iframe 边界保持各自职责。

Web 工程约束见 [web/AGENTS.md](web/AGENTS.md)。独立插件 iframe 内部的组件库由各插件维护，Launcher 使用自己的原生组件体系。

**Key Characteristics:**

- 白色页面、浅灰盒子与柔和阴影，不使用渐变、背景模糊或半透明材质。
- 蓝色主操作、链接与焦点，以及按亮暗主题切换原版与反色版的黑白人物标识。
- 随包的 HarmonyOS Sans SC 用于 Web 与 Launcher 的全部界面文字；字号、字重与间距形成克制层级。
- 控件取所在容器相反的明暗：页面上是浅灰，盒子里是白色，都以轻阴影抬起。
- 亮暗主题与键盘操作保持等价操作能力；桌面以 16:9 的 1920×1080 为最低分辨率与界面验证基准。

本文件的 token 前置数据由 [design/tokens.json](design/tokens.json) 生成。它是共享主题 token 的唯一机器值来源，采用 base → semantic light/dark → component 结构；[生成脚本](scripts/generate-design-tokens.mjs) 同时维护 Web、Launcher、favicon、共享字体 CSS 与 [.impeccable/design.json](.impeccable/design.json)。运行 `node scripts/generate-design-tokens.mjs` 更新生成物，运行 `node scripts/generate-design-tokens.mjs --check` 校验生成物漂移与指定对比度。原生图标由独立的 [图标生成脚本](scripts/generate-launcher-icons.mjs) 维护。

前置数据、共享字体 CSS 与 sidecar 均由生成器维护，不直接编辑；sidecar 的 narrative 提取概述、关键特征、命名规则及 Do/Don't 列表。sidecar 的通用组件预览表达共享基础，Web 产品组件的尺寸以组件源码为准，焦点与浮层生命周期以本文对应规则为准。应用局部映射在正文与界面规范中说明，不改变共享基础 token 的含义。

## Colors

白色页面是画布，浅灰是盒子，白色是盒子里的控件；暗色主题使用近黑画布、深灰盒子与更亮一级的控件面。蓝色是唯一的交互强调色，只出现在主操作、链接、焦点、选中导航图标与选中淡面中。

### Primary

- **蓝色主操作**：浅色使用 light-primary 配白色文字，暗色使用 dark-primary；悬停与按下分别消费对应组件映射。
- **蓝色品牌前景**：链接、焦点、选中导航图标与勾选标记消费品牌角色，不扩散到普通正文和容器边界；选中淡面（brand-soft）是淡蓝，用于选中行与选中选项。人物标识使用原版或反色版的黑白配色。

### Neutral

- **白色画布与浅灰盒子**：页面是画布；卡片、侧栏、对话框、抽屉、菜单与提示条是浅灰表面，只以阴影与页面分开。
- **白色控件面与凹槽**：盒子里的控件、选中导航胶囊与所有输入框使用抬升表面；分段控件轨道、页面上控件的悬停与中性图标底使用凹槽色。
- **近黑与深灰**：暗色画布、盒子与导航，控件面比盒子亮一级。
- **中性悬停与焦点**：导航悬停与普通悬停面使用中性角色；键盘焦点使用品牌前景色，字段在现有边界内显示焦点，见 Inputs / Fields。
- **正文、辅文与边界**：分隔线负责盒子内的分组；控件边界负责输入框等可操作目标，两者不能互换。

### Named Rules

**The Semantic Token Rule.** 运行代码只消费语义和组件 token；基础色阶仅用于建立映射。

**The Neutral Ground Rule.** 普通表面、分隔线、文字与悬停背景保持中性；蓝色用于主操作、链接、焦点、选中导航图标与选中淡面，不用于大面积背景。

**The Semantic Independence Rule.** 人工关注、成功、警告和危险保持独立，状态同时提供文字、图标或结构化标签。

人工关注用于确认、可信代码提示和未保存草稿；警告表达降级，危险表达失败、阻塞或破坏性操作。恢复兼容、降级、阻塞分别使用 success、warning、danger，文字与图形使用同一语义。

## Typography

**Display Font:** HarmonyOS Sans SC，回退为 Microsoft YaHei UI、PingFang SC 与 sans-serif，用于品牌文字、页面标题和面板标题。共享 [typography.generated.css](design/typography.generated.css) 以 `@font-face` 声明 [design/fonts/harmonyos-sans-sc](design/fonts/harmonyos-sans-sc/upstream.json) 中未经修改的 Regular、Medium、Bold 三个 TTF，两端随构建打包；[字体许可协议](design/fonts/harmonyos-sans-sc/LICENSE.txt) 随两端公开资源附带，两端界面都声明使用了该字体。

**Body Font:** 与标题相同的 HarmonyOS Sans SC，回退为 Microsoft YaHei UI、PingFang SC、Hiragino Sans GB 与系统无衬线字体。Web 管理面、认证页与 Launcher 的正文和标准控件都继承 `--font-sans`，Launcher 的 Fluent `fontFamilyBase` 同样映射到该变量。字体只提供 400、500、700 三个字重，600 及以上的字重使用 Bold。

**Label/Mono Font:** 标签沿用所在应用的正文栈。Web 日志行的时间、来源与技术元数据，详情中的来源、插件 ID、请求 ID，以及结构化数据、JSON 和代码使用 Cascadia Mono、Consolas、JetBrains Mono 等宽回退栈；日志消息正文使用界面字体，不因位于 `pre` 中而改用等宽字体。

### Hierarchy

- **Headline**：管理页面标题保持紧凑，使用前置 token 的标题级；认证面板标题使用局部字号。
- **Title / Section**：分区、面板与组标题。
- **Body**：正文与标准控件。
- **Label / Mono**：标签、表头与技术元数据。
- 辅助元数据使用最小一级字号；关键操作不用该级字号。少量真实主状态可以使用 token 中较大的字号级，不形成巨型指标区。
- 连续说明正文限制行宽；表格、日志和技术工作区按内容需要延展。

### Named Rules

**The Quiet Hierarchy Rule.** 层级依靠字号、字重和间距建立，不通过装饰性眉题、渐变文字或巨型指标制造噪音。

## Layout

Web 只提供桌面布局，使用可收起的持久导航与单一工作区，管理壳没有独立的顶栏：每页第一行由页面组件提供标题、说明、页面搜索与本页操作。侧栏底部依次是账号菜单、主题切换与折叠按钮，账号菜单收纳修改账户、偏好设置、全屏、退出登录与关闭服务；软件名后显示构建版本，偏好从右侧抽屉打开。Launcher 窗口按可用工作区与最小尺寸约束调整，包含透明标题栏、悬浮的带文字导航盒子与单一主工作区。桌面以 16:9 的 1920×1080 为最低分辨率与界面验证基准，手机、平板与更低的分辨率不提供专门布局；Launcher 在该屏幕上以默认 1280×720 窗口验证。尺寸以组件代码为准，具体界面规则见 [Web](docs/design/web-management-ui.md) 与 [Launcher](docs/design/launcher-design-system.md)。

间距使用前置 token 的 xs 至 xxl 标尺。独立任务使用浅灰盒子，同一任务内的字段、日志和数据行通过间距与分隔线组织。页面主操作位于页面首行的稳定位置，状态页先汇总需要处理的事项再按对象分组，列表按真实内容排列。

插件集合与协议连接使用独立对象卡片网格，同排卡片等高、操作栏底部对齐，长元数据与健康提示允许内容自然增高；不通过裁掉状态或缩小触控目标保证固定数量。插件筛选、全局插件设置、治理表单、通用配置工作台与协议中心弹窗的布局和保存入口见 [Web 界面规范](docs/design/web-management-ui.md)。

页面在桌面视口内通过换行与自然增高容纳内容，并保留必要纵向滚动；更宽的屏幕只增加网格列数，不切换导航或列表结构。技术表、长路径与日志可以在自身区域横向查看，普通页面不依赖整页横向滚动。正文和关键控件不随视口任意缩小，粗指针环境使用触控尺寸的交互目标。

## Elevation & Depth

页面、盒子与控件三级明暗配合柔和阴影提供主要层级：盒子以表面阴影抬起，盒子里的小控件、选中导航与分段滑块以 raised 阴影抬起，菜单、选择器浮层、说明弹层、抽屉、Dialog 与 Toast 以浮层阴影抬起。所有表面不透明，不使用渐变、背景模糊或半透明材质；两套主题的阴影与 sticky、menu、drawer、modal、toast、emergency 层级由 sidecar 记录。

Web 产品弹窗、抽屉、菜单、说明弹层与选择器浮层使用浅灰表面并消费浮层阴影。遮罩是纯色压暗：浅色主题为 28% 的黑色，暗色主题为 50% 的黑色。Tooltip 使用正文色作底、表面色作文字，保持独立的高对比提示。

**The Web Overlay Stack Rule.** Web 弹窗与抽屉遮罩从 1200 起按打开顺序递增 20，内容位于所属遮罩上方 1 层；嵌套菜单、说明弹层和选择器继承所属层级再加 5，Tooltip 加 8。未嵌套菜单、说明弹层、选择器和 Tooltip 的基准为 1100，Toast 为 1600，使退出中的菜单留在新打开的抽屉下方。这些 Web 局部层级不改变共享基础层级或 Launcher。

认证入口与管理工作区使用同一套表面：白色页面上的浅灰面板、白色字段与蓝色主按钮，颜色由认证主题 token 从共享 token 映射，入口规则见 [Web 认证规范](docs/design/web-management-ui.md#认证入口)。

Launcher 与 Web 管理面使用同一套表面规则：窗口是白色画布（暗色主题为炭黑），侧栏、内容分组、主题菜单与确认对话框是浅灰盒子，只用柔和阴影分层；控件取所在容器相反的明暗，主操作使用蓝色填充，不使用渐变、背景模糊或半透明材质。规则与强制颜色降级见 [Launcher 界面规范](docs/design/launcher-design-system.md#表面与层次)。

forced-colors 下盒子、浮层与认证面板使用系统颜色与 1px 系统边界并去掉阴影，内容可读性与操作反馈不依赖阴影。

### Named Rules

**The Single Elevation Rule.** 每个盒子只用一种方式与背景分开：盒子用阴影，不再叠加边框；盒子内的分组用分隔线，嵌套在盒子里的列表与视口不叠加第二层盒子。盒子里需要与正文区分的代码、用法与实时输出使用白色内嵌底，同样不加边框。

**The Opposite Fill Rule.** 控件取所在容器相反的明暗：位于白色页面上的次要按钮与分段轨道使用浅灰，位于浅灰盒子里的使用白色，两者都以 raised 阴影抬起；输入框始终为白色并保留控件边界。

## Shapes

标准控件使用温和圆角（md），独立任务表面与浮层采用较宽圆角（lg）；紧凑组件使用小圆角（xs / sm），状态胶囊使用 full。认证面板、凭据输入和主按钮使用更大的局部圆角；Web 与 Launcher 的侧栏盒子使用大圆角，按钮与导航项为胶囊，状态标识为圆形。这些局部圆角不作为通用控件标准。

Web 产品按钮与导航项为胶囊，输入框与选择器使用 md 圆角，卡片使用 xl 圆角，居中产品弹窗采用 xxl 圆角。配置、搜索与确认弹窗共享该形状，内部字段通过间距与分隔线分组。左右抽屉是悬浮在视口内、四角圆角的面板；底部抽屉只有上方两个角为圆角。菜单与 Toast 使用 lg 圆角，Tooltip 使用现有 md 圆角，Web 状态与分类标签使用紧凑的 sm 圆角。

品牌标识为黑白人物，保留帽子、蝴蝶结、手势与完整曲线轮廓，以 [design/mark.json](design/mark.json) 为唯一母版。母版保留原始 `0 0 1254 1254` viewBox 与两个使用 `evenodd` 填充的贝塞尔复合路径；黑白区域均不透明，人物之外保留透明背景。原版为白发，母版的 `outline` 沿第一个路径绘制白色细轮廓，位于黑白填充后方。Web、Launcher 的界面标识与 Web favicon 在浅色主题使用原版，在暗色主题使用用户确认的整体反色版：填充与描边同步黑白互换，曲线、手势和透明背景保持一致。Launcher 功能图标使用 Fluent Regular，品牌标识不承担操作或状态含义。

原生资产位于 [launcher/assets/](launcher/assets/)，应用 PNG、亮暗托盘 PNG 和 Windows ICO 由 [图标生成脚本](scripts/generate-launcher-icons.mjs) 通过 Skia 后端的 Canvas 2D（`@napi-rs/canvas`）与 `Path2D` 直接从母版确定性光栅渲染，保留同一人物曲线与透明背景。应用 PNG 与 Windows ICO 固定使用原版；托盘按系统主题使用原版或整体反色版。标识源自经用户确认的 AI 辅助人物概念图，再转为贝塞尔矢量；PNG 内嵌对应来源元数据。ICO 包含 16、24、32、48、64、128、256px 图像，每个尺寸独立渲染而非缩放。运行 `node scripts/generate-launcher-icons.mjs --check` 校验来源与资产摘要；Windows 构建资源的生成与验证入口见 Launcher 界面规范。

## Components

### Buttons

主按钮只强调当前工作流的主要动作，使用蓝色填充、白色文字与胶囊形。次级操作使用与所在容器明暗相反的控件填充与 raised 阴影，不加边框；人工关注和危险操作各用独立语义。默认、悬停、焦点、按下、禁用和加载状态均保留明确反馈。按钮高度由组件 token 提供，粗指针环境使用触控尺寸目标。Launcher 在管理面可用时以打开管理面为主操作，否则提供启动操作；停止操作使用危险次级样式并遵循确认规则。Launcher 各工作区的主操作是蓝色胶囊，次级、工具与危险操作是控件填充的胶囊，均以外侧轮廓显示键盘焦点，禁用时收起阴影并弱化文字。

Web 使用 [`AppButton`](web/src/components/AppButton.vue)：默认样式为次级按钮，主要动作显式使用蓝色填充，幽灵按钮悬停时显示中性悬停面；危险动作使用淡危险底色与危险文字。默认和图标按钮高度一致，粗指针下使用触控尺寸。加载状态保留动作文字，同时禁用重复提交并显示忙碌语义；减少动态效果或强制颜色时保留静态反馈。

### Inputs / Fields

业务字段与认证字段都使用白色控件面，保留完整控件边界和持续可见标签。错误说明关联字段，禁用状态保持可读，占位文本不承担标签职责。Web 字段焦点由原有边框和内侧描边组成，不向控件外扩张；认证字段使用同样的几何与所属面板的颜色。forced-colors 下使用内侧系统焦点轮廓。

Web 使用 [`AppField`](web/src/components/AppField.vue) 关联持续可见标签、控件和错误说明，标签与说明沿用正文与标签字号层级。[`AppInput`](web/src/components/AppInput.vue) 与 [`AppSelect`](web/src/components/AppSelect.vue) 在粗指针下使用触控尺寸；多选文字可换行并自然增高。密码显示开关保留可访问名称和按下状态，错误通过文字与 `aria-invalid` 一起表达；支持清空的输入框在清空后保留输入焦点。认证字段使用 Authentication 中的局部尺寸。

独立编辑字段通过 `AppField floating` 使用框内浮动标签，取消重复的框外标题。标签在空值且未聚焦时位于框内；聚焦、已有值或自动填充时上移为小号标签，输入内容位于其下。说明与错误留在控件下方，密码显示按钮、前缀图标和必填语义继续保留。登录、初始化、账户修改、协议连接、插件安装与来源编辑、全局插件设置中的普通字段，以及日志的单值筛选采用此模式。配置工作台的左右说明行、复合限流、开关、多值条目、多选筛选和原生日期时间范围保持外置标签。

AppInput 的布局容器样式与内层字段样式分开，前缀图标不接收指针操作，也不替代标签。AppSelect 保留字符串、数字和布尔值的原类型，尚未包含在选项中的已选值继续显示；异步选项到达后更新显示名称。可清除的多选在清除后将焦点归还选择器。[`AppNumberInput`](web/src/components/AppNumberInput.vue) 按业务需要启用可空值，不将空白输入自动当作零；限流字段分别标注次数、时间窗和单位。

按需加载选项的筛选器同时响应获得焦点和选择器实际打开；AppSelect 提供打开事件，使鼠标打开未触发原生 focus 时仍能读取选项。日志插件筛选继续抑制重复请求，保留多值与未知插件 ID，协议选择“全部”会清除协议条件。

[`AppTextarea`](web/src/components/AppTextarea.vue) 通过行数和最大行数控制可用高度，允许纵向调整。标签输入使用 [`AppTagsInput`](web/src/components/AppTagsInput.vue)，条目可换行，每个删除入口有可访问名称；[`AppSearchInput`](web/src/components/AppSearchInput.vue) 通过 Enter 或搜索按钮显式提交查询，清空输入保留编辑焦点。

**The Tag Delimiter Rule.** 标签输入通过 Enter 或离开输入框确认条目，只按业务显式提供的分隔符拆分输入与粘贴。全局指令前缀保留字面逗号；菜单中心按其既有字段规则使用逗号、中文逗号和空格分隔，不把该规则扩散到其他标签字段。

**The Web Product Focus Rule.** Web 字段通过边界着色与内侧描边显示焦点；错误字段使用危险语义。按钮、导航、页签和分段选择使用内侧轮廓，偏移统一由 `--focus-outline-offset` 提供；实心主按钮使用对应前景色。基础控件不使用向外扩张的焦点或错误光环，不叠加多层轮廓；字段容器不另加聚焦外框。强制颜色模式保留系统可见焦点，Launcher 遵循自身平台规范。

### Navigation

当前项是带 raised 阴影的选中胶囊，文字加粗、功能图标使用品牌前景色，不依赖细侧边条或品牌标识表达当前位置。Web 侧栏使用原生导航按钮，分类保留展开与收起；插件中心从顶级入口进入持久侧栏层，按管理、工具与已安装插件分组展示固定页面和插件资源。返回主导航只改变导航层级，保留当前页面；收起侧栏通过菜单提供固定入口。

**The Sidebar Resource Rule.** 插件资源的打开按钮与展开按钮为同一行中的兄弟控件，分别负责导航与展开管理页，具有独立的可访问名称和焦点；不嵌套可交互按钮。插件可独立展开概览和 manifest 声明的管理页，展开状态、加载反馈与失败重试围绕对应资源呈现。

管理壳只有一个工作区，切换页面直接替换当前内容，页面缓存规则见 [Web 界面规范](docs/design/web-management-ui.md)。保存、刷新等业务操作与对应字段或工具栏相邻。

### Tabs and segmented controls

[`AppTabs`](web/src/components/AppTabs.vue) 用于抽屉内等局部分区，采用 Reka 自动激活与触控尺寸的标签目标；选中项使用品牌前景色文字和底线，标签列表可横向滚动。[`AppSegmented`](web/src/components/AppSegmented.vue) 用于主题、密度、页面切换、内容宽度与状态页检查类型等单选：等宽选项排列在凹槽色的胶囊轨道上，选中项是带 raised 阴影的白色滑块并加粗文字，粗指针下使用触控尺寸。两者保留禁用和可见键盘焦点。

AppTabs 的标签可附带数量标记，标题区的额外操作承载当前分区筛选。菜单预览和插件详情控制台显式启用 keepAlive，切换时隐藏内容并保留节点；普通局部分区按实际需要决定是否常驻，不以重建预览或控制台来完成视觉切换。

### Menus and transient feedback

[`AppDropdown`](web/src/components/AppDropdown.vue) 统一按钮菜单，宽度随内容限制在合理范围内并保留视口碰撞余量；超长菜单在自身区域滚动。菜单项高亮是带轻阴影的白色胶囊，危险动作使用独立危险语义，粗指针下使用触控尺寸目标。主题菜单在 system、light、dark 之间选择，并以文字和勾选标记共同表达当前偏好。

[`AppTooltip`](web/src/components/AppTooltip.vue) 在 450ms 延迟后提供简短补充说明，宽度随内容展开并受视口限制；普通短标签不压成单字竖排，多行配置帮助按内容换行。提示不替代触发器的可访问名称。搜索使用居中弹窗，打开后聚焦输入框，结果展示页面名称与路径，支持上下选择、Enter 导航和 Escape 关闭。

[`AppPopover`](web/src/components/AppPopover.vue) 通过点击打开说明或紧凑筛选表单，支持受控开关、方向、对齐和目标宽度。默认位于触发器下方并左对齐，宽度不超过视口。内容使用浅灰浮层表面、紧凑正文与可选标题，层级遵循共享 Web 浮层规则；持续可见的标签和必要字段反馈仍留在表单内。

[`AppToastHost`](web/src/components/AppToastHost.vue) 在右上方显示最多四条即时反馈，通知限制宽度并保留视口边距。每条提示包含语义图标、可换行正文和手动关闭入口；普通提示停留 4.5 秒，错误提示为 7 秒，关闭后保留 160ms 退场。持续问题留在页面状态中，不依赖短暂 Toast 承载。[`AppSpinner`](web/src/components/AppSpinner.vue) 提供状态文字或辅助技术可读名称，[`AppSkeleton`](web/src/components/AppSkeleton.vue) 提供忙碌语义；reduced-motion 或 forced-colors 下停止旋转、脉冲和提示过渡。

### Chips / Status

Web 持续提示使用 [`AppAlert`](web/src/components/AppAlert.vue) 的紧凑无底色状态行，语义色保留在图标和小型标签，说明与操作紧邻对应内容。加载失败通过 [`RetryPanel`](web/src/components/RetryPanel.vue) 在工作区内居中显示原因与中性重试按钮；不使用横贯工作区的彩色底面、外框或横幅。诊断与恢复事项使用中性分隔线组织。

状态标签同时呈现文字或图标，不能只显示色点。标签表达状态和筛选，不替代操作按钮。关注提示、异常和空态提供原因、影响、可执行动作或必要前置条件。日志无匹配结果时说明为空，并提供调整筛选或等待新日志的方向。

系统状态页先汇总需要人工处理的事项，再以协议连接、运行信息、检查与近期变化四个盒子呈现系统，状态页不展示插件；复查与运行环境准备使用独立操作。通用故障页通过 [`AppFallback`](web/src/components/fallback/AppFallback.vue) 复用返回首页与重试控件，重试期间保留忙碌反馈。

[`AppStatusTag`](web/src/components/AppStatusTag.vue) 使用 AppBadge 的语义颜色、文字和辅助圆点表达状态；[`AppTag`](web/src/components/AppTag.vue) 关闭圆点，用于指令、分类、权限和数量等紧凑信息。别名、权限与来源不因使用同一标签外形而被解释为运行状态。

### Cards / Containers

容器是浅灰盒子，以柔和阴影与页面分开，正文按自然高度排列。字段组不层层包成卡片，嵌套在盒子里的列表与视口不叠加第二层盒子；日志只保留一个外框，其内部使用行分隔与独立正文滚动区。

[`AppCard`](web/src/components/AppCard.vue) 是浅灰盒子，以表面阴影抬起、不加边框，标题区与正文通过分隔线区分。正文提供默认与紧凑两种内边距；flat 分区保持透明背景，highlight 用人工关注语义表达需要判断的内容。重试面板保留原因说明和直接操作，管理上下文操作使用可换行的按钮组。

插件集合是有任务意义的对象卡片布局，不要求其他数据页采用卡片；卡片构成、图标回退、操作入口与点击目标见 [Web 界面规范](docs/design/web-management-ui.md)。

插件启停按钮继续使用 switch 语义，读出当前状态和下一步动作，忙碌时禁用重复操作。详情中的控制台保留动态行高虚拟列表、底部跟随、筛选和清空入口，流向、级别与请求标识紧邻对应正文。指令面板展示有效名称、别名、冲突、用法和权限，不把全部别名挤入插件集合卡片。

协议连接卡片以已连接账号为主体，连接标识不代替平台账号；已保存配置与当前运行状态分别说明，需要重启的变更在列表上方提示，卡片不将保存成功推断为连接已运行。头像、尺寸、布局与操作入口见 [Web 界面规范](docs/design/web-management-ui.md)。

### Data tables and details

[`AppDataTable`](web/src/components/AppDataTable.vue) 使用原生 table、列标题和辅助技术可读的表名；列定义提供标识、标签、宽度和对齐，业务单元格保留自己的内容与操作。表格使用紧凑正文、中性表头和行分隔，长内容可换行，宽表只在内部区域横向滚动；首次加载展示骨架，空数据提供对应空态。

[`AppDetails`](web/src/components/AppDetails.vue) 与 [`AppDetailItem`](web/src/components/AppDetailItem.vue) 使用定义列表呈现安装检查、来源和属性等键值信息。各行直接排在所属盒子上，不另加外框；标签列使用次要文字色，值列允许路径、摘要和长标识换行，各项通过分隔线组织。

名单表格保留最小宽度并在自己的区域横向滚动；行内新增保留类型、目标、说明与操作。字段错误通过 `aria-invalid`、关联说明和可读错误文字一起表达。复制入口预留图标空间，以透明度反馈状态，避免复制前后改变列宽。

调度任务使用原生表格，右侧操作列固定在自身滚动区内。任务详情使用居中 AppDialog，错误说明通过 AppPopover 查看，状态统计条随数据直接更新。

### Logs and diagnostic detail

实时与历史日志保持各自的列表、筛选和滚动职责。高级筛选通过受控说明弹层编辑协议、插件多选和请求标识，历史范围使用本地日期时间输入。清除筛选、分页、底部跟随和手动滚动沿用现有工作区状态，持续新增日志不逐条播放入场动画。

**The Log Row Density Rule.** 实时与历史日志的行级标签使用 AppTag 的 small 尺寸。虚拟列表以常规行高作为估算值，并测量实际行高；换行正文允许自然增高，虚拟列表维护滚动锚点与底部跟随。

[`ManagementLogDetailDrawer`](web/src/components/logs/ManagementLogDetailDrawer.vue) 在宿主尺寸可用时呈现非模态桌面窗口，位置与拖动范围按宿主可用尺寸约束。页头使用普通二级标题“日志详情”，来源、级别、协议和时间排列在其下；正文在窗口内滚动，原日志列表继续可操作。缺少有效宿主尺寸时使用右侧 AppDrawer，并沿用模态抽屉的退出和焦点规则。

**The Log Detail Exit Rule.** 日志详情的展示层保留关闭前最后一份摘要、正文、加载或错误内容，直到桌面窗口的 after-leave 或回退抽屉的 afterClose 完成后清理；控制器继续独立管理正式选中状态与请求缓存。关闭后恢复到仍有效的日志行；非模态桌面窗口不夺走用户已转移到其他控件的焦点。

### Web product dialogs and progressive fields

Web 产品组件基于 Vue 3、Reka UI 2.10.4、仓库持有的 shadcn-vue / reka-nova 源码、Tailwind CSS 4 与 motion-v 2.4.2，工程版本以 [package.json](web/package.json) 为准。业务页面复用 App 组件层；底层交互语义由 Reka UI 承担，主题映射集中在 [tailwind.css](web/src/styles/tailwind.css)，浮层动效集中在 [presets.ts](web/src/motion/presets.ts)。

协议中心的“添加连接”先在配置弹窗内展示协议名称与说明，选择后在同一弹窗填写配置；“更换协议”返回选择步骤。常用地址、凭据、接收消息和启用状态优先展示，连接标识、沙箱、共用重连策略、令牌兼容选项与运行诊断按适用条件逐级展开。高级字段出错时展开对应区域并聚焦错误控件。保存和取消留在固定页脚，正文独立滚动；共享重连参数明确说明影响所有连接。

[`AppDialog`](web/src/components/AppDialog.vue) 的居中模式以 CSS 固定定位保持视口中心，四周保留视口边距。标题、说明与关闭按钮留在页头，操作留在页脚；正文长度变化时按实测内容高度过渡，达到视口上限后只滚动正文。

[`AppDrawer`](web/src/components/AppDrawer.vue) 复用 AppDialog 的左侧、右侧或底部呈现。左右抽屉是悬浮在视口内的面板，高度为视口高度减去上下间距，宽度受视口限制；正文填充余下高度并独立滚动，页头与页脚保持可达。底部呈现贴齐视口底边、左右铺满，高度随实测内容变化，不超过视口高度。只有左右抽屉固定高度，居中弹窗和底部面板继续由 Motion 管理实测高度，避免不同呈现互相覆盖定位和动画样式。

**The Web Dialog Lifecycle Rule.** AppDialog 与 AppDrawer 在关闭动画完成前保留 Reka 内容、遮罩、焦点约束与业务内容；由 Motion 的 animationComplete 完成退场后再释放层级、触发 afterClose 并归还焦点，调用方不得随 open 变为 false 提前卸载内容。打开时保存明确的备用焦点入口；原触发项已卸载，或 afterClose 已清理删除候选对象时，仍归还到该有效入口。嵌套确认和选择器服从所属层级；确认弹窗使用 alertdialog，并将初始焦点放在取消操作。忙碌状态阻止关闭与重复提交，未保存修改通过确认弹窗处理。

插件安装检查与商店安装确认保留各自已有的检查信息，等待 afterClose 再清理，避免退场期间出现空内容。商店源编辑器配置有效的备用焦点入口；源删除使用居中的嵌套确认，取消只关闭确认，不写入删除操作。

账号删除和名单删除同样使用居中确认，候选对象在退出完成后清理；删除成功后可回到对应平台或名单的新增入口。启用空白名单时说明影响并要求确认，取消确认不改变启用状态。

### Authentication

登录、首次初始化与凭据恢复指引共享居中单栏面板，保留共享黑白人物标识与 HarmonyOS Sans SC，凭据表单使用 AppField、AppInput、AppButton 和 AppAlert。面板是白色页面上的浅灰盒子，以柔和阴影抬起，不使用背景图片或背景动画。

入场动画、reduced-motion 与 forced-colors 行为见 [Web 认证规范](docs/design/web-management-ui.md#认证入口)。认证区域文字选区使用现有品牌填充与对应前景。

[`AuthCredentialsForm`](web/src/components/auth/AuthCredentialsForm.vue) 的标签、输入与错误保持关联，账号和密码使用相同控件高度与局部焦点样式。密码可见性按钮具有可访问名称和按下状态；提交期间输入、显示开关和提交按钮均禁用，字段校验失败时聚焦首个错误输入。认证专用 CSS 变量由 [preferences/auth.ts](web/src/preferences/auth.ts) 映射，表面样式保持在 AuthLayout 与认证组件内。

“忘记密钥？”在登录面板内打开本机重置指引，返回时保留已填凭据并将焦点归还入口；重置由 Launcher 或停服后的 CLI 完成。入口、步骤与字段反馈见 [Web 认证规范](docs/design/web-management-ui.md#认证入口)。

### Motion and ownership

控件反馈采用 100–160ms 的短节奏，工作区采用 180–220ms，浮层采用 200–220ms；当前共有反馈、Web 内容切换、Launcher 工作区和浮层分别使用 160ms、200ms、220ms 和 220ms。动画主要改变 opacity / transform 或控件状态属性，服务于选择、层级切换和显隐，持续日志不逐条播放进入动画。Web 居中弹窗和底部面板另对实测内容高度进行过渡，以保持字段展开和异步内容变化时的定位关系。

AppDialog 与 AppDrawer 入场为 220ms、退场为 160ms，沿用现有缓动 `cubic-bezier(0.16, 1, 0.3, 1)`。居中模式改变透明度与缩放（0.96 ↔ 1），CSS 定位负责居中，Motion 负责显隐和实测高度；左右抽屉保持缩放为 1，以透明度和短距离水平位移表现打开方向，底部面板使用短距离垂直位移。reduced-motion 或 forced-colors 下时长为零、缩放为 1、位移为零，同时保留完整关闭生命周期与焦点归还；forced-colors 下保留系统表面边界。

Web 工作区优先使用只捕获主内容的 View Transition，缺少能力时由统一的 Motion for Vue 入口降级，侧栏保持可交互。菜单通过自身 CSS 状态过渡显隐（160ms），Toast 沿用同组过渡（进入 200ms、退出 160ms）；这些元素不叠加第二套 Motion 动画，减少动态效果或强制颜色时即时呈现。

Web 与 Launcher 的主题切换由 Motion 驱动新主题快照，从主题入口以 420ms 圆形展开；没有入口坐标时以 280ms 淡入。账户修改复用 AppDialog 的进入、取消退出、内容高度与焦点生命周期，可选用户名按需披露。两者在减少动态效果或强制颜色模式下即时呈现。

非模态桌面日志窗口保留自身 CSS 透明度与水平位移过渡（220ms），条目间的详情切换使用 160ms 淡化；它不叠加 AppDialog 的缩放和焦点锁。共享 reduced-motion 与 forced-colors 样式覆盖该窗口过渡。

Launcher 动效由 Motion 驱动：工作区以 220ms 从 0.88 不透明度淡入且不位移，确认对话框进入为 220ms、退出为 160ms，退出完成后才卸载，主题菜单使用 Fluent Menu 默认的弹层动效；服务状态变化时状态圆点底色、图标与状态文字交叉过渡，启动中与停止中的同步图标缓慢旋转；可展开区域在 220ms 内展开、160ms 内收起；按钮按下时换用按下填充并收起阴影；侧栏选中背景以弹簧滑到新导航项；状态与内容在点击时更新，连续切换取消旧动画并从当前位置继续。导航持续可交互，reduced-motion 或 forced-colors 下立即完成。Web 与 Launcher 均保留 system、light、dark 主题偏好，手动选择可持久化，不使用按时钟自动切换配置。动效时长不代表帧率或性能承诺。

**The Single Motion Owner Rule.** 同一元素只接受一种动效机制，连续操作取消旧动画并以最新状态为准。

**The Brand Mark Rule.** 界面与托盘标识随所属主题使用原版或整体反色版，保留同一母版的曲线、手势和透明背景；静态应用文件图标使用原版。品牌不代替状态图标或导航文字。

插件页面、聊天卡片与渲染模板拥有独立内容和样式边界，视觉体系由各插件仓库自行维护；其管理面 Host 使用本体系。独立 iframe 不继承宿主 CSS、字体或组件运行时，内部页面保留自身组件库；页面与宿主同源加载，Vue SDK 可选地把宿主主题变量提供给页面，详见 [插件管理面](docs/design/plugin-management-surface.md)。

**The Embedded Content Rule.** 需要连续工作的预览和控制台通过 AppTabs 的 keepAlive 保留隐藏节点；插件管理面 Host 使用 AppLoadingPanel 表达忙碌并暂时阻止内容交互，不因加载提示重建 iframe。Host 管理错误恢复和显式重载，独立插件页面管理自己的内部组件。

[`PluginManagementUIHost`](web/src/components/plugins/PluginManagementUIHost.vue) 保留现有 iframe 高度同步与 160ms CSS 高度过渡，不与 Motion 叠加；reduced-motion 和 forced-colors 由共享响应式样式将该过渡压缩为即时呈现。

**The Template Preview Scale Rule.** [`TemplatePreviewFrame`](web/src/components/templates/TemplatePreviewFrame.vue) 保留实际固定宽度的预览文档，外层按缩放后的宽度居中，内层 iframe 从左上角缩放。调整宿主或视口尺寸只更新预览布局，保留同一 iframe 文档，避免缩小后偏移裁切；模板内容、数据与资源仍由既有预览流程更新。

## Do's and Don'ts

### Do:

- Do 用中性导航、连续工作区和精确分隔线建立稳定方位。
- Do 将蓝色留给主操作、链接、焦点与选中导航图标，保持普通表面与文字中性。
- Do 用阴影抬起浅灰盒子，并让控件取所在容器相反的明暗。
- Do 保持亮暗主题的信息层级、状态含义与操作能力等价。
- Do 使用项目 token、共享标题字体与所属应用的标准产品组件，遵守 Web、Launcher 和独立插件内容的边界。
- Do 保持可见焦点、键盘操作、触控目标、reduced-motion 与 forced-colors 支持。
- Do 让真实数据、必要警告和用户任务决定内容高度与页面密度。
- Do 以 16:9 的 1920×1080 为最低桌面分辨率完成界面验证。

### Don't:

- Don't 使用霓虹色、发光边界或通用深色科技仪表盘作为品牌语言。
- Don't 使用巨型标题、装饰性眉题、hero 指标模板或虚构数据填满页面。
- Don't 使用无任务意义的同尺寸卡片拼贴、多层日志外框或无任务边界的嵌套卡片；独立插件集合与协议连接卡片属于明确保留的对象布局。
- Don't 在盒子上同时叠加边框与阴影，也不使用渐变、背景模糊、毛玻璃或半透明材质，不逐条动画日志。
- Don't 依赖颜色单独表达状态，或用 attention 混同 warning 与 danger。
- Don't 在 Web 业务页面新增另一套控件或浮层行为；统一复用已有产品组件。
- Don't 为视觉风格引入运行时主题服务或跨 iframe 样式注入。
