---
name: RayleaBot
description: 深梅紫应用壳与中性高密度工作区组成的自托管机器人管理界面
token-source: design/tokens.json
token-format: DTCG 2025.10 compatible
north-star: 梅紫仪表盘
colors:
  plum-50: "#FFF4F9"
  plum-100: "#FBE8F2"
  plum-200: "#F4CEE0"
  plum-300: "#E8AAC7"
  plum-400: "#D57BA6"
  plum-500: "#BF4F87"
  plum-600: "#9F356C"
  plum-700: "#8A285D"
  plum-800: "#6C204B"
  plum-900: "#3F1830"
  plum-1000: "#27101E"
  light-canvas: "#F6F3F5"
  light-surface: "#FFFBFD"
  light-text: "#211820"
  light-text-muted: "#6B5E66"
  light-border: "#DDD4D9"
  light-control-border: "#93868E"
  light-primary: "#8A285D"
  light-focus: "#9F356C"
  light-chrome: "#3F1830"
  dark-canvas: "#151114"
  dark-surface: "#1D181C"
  dark-text: "#F4EDF1"
  dark-text-muted: "#BAADB4"
  dark-border: "#41363D"
  dark-control-border: "#806C78"
  dark-primary: "#D57BA6"
  dark-focus: "#F0A4C9"
  dark-chrome: "#24151F"
typography:
  sizes: "12 / 13 / 14 / 16 / 18 / 24 / 30px"
rounded:
  scale: "4 / 6 / 10 / 14 / 999px"
spacing:
  scale: "4 / 8 / 12 / 16 / 24 / 32px"
layers:
  scale: "sticky 100 / menu 200 / drawer 300 / modal 400 / toast 500 / emergency 600"
---

# RayleaBot 设计体系

## 设计北极星

RayleaBot 的设计北极星是“梅紫仪表盘”：深梅紫应用壳提供稳定、可辨认的方位感，中性工作区承载高密度管理任务，原创“定位器”标识把品牌、导航选择和真实运行状态连接成同一视觉语言。

界面首先服务于诊断、配置、恢复和长期运行。信息层级依靠字号、间距、表面与完整边界建立；品牌色集中在应用壳、焦点和当前工作流的主操作，不把每个容器都做成品牌色卡片。

关键特征：

- 深梅紫应用壳与中性工作区形成稳定分区。
- Web 克制使用品牌色；Launcher 的标题栏与导航轨具有更明确的品牌识别。
- 亮暗主题提供等价的信息层级、状态语义和操作能力。
- 定位器只出现在品牌、当前导航和真实主任务状态，不作为无意义装饰。
- 标准控件、清晰文案和可见焦点优先于装饰性效果。

## Token 契约

`design/tokens.json` 是唯一机器值源，结构固定为 `base → semantic light/dark → component`。运行代码只消费语义或组件 token，不直接消费基础色阶。

生成命令：

```bash
node scripts/generate-design-tokens.mjs
node scripts/generate-design-tokens.mjs --check
```

生成物包括 Web TypeScript、Web SCSS、Launcher TypeScript、本文件前置元数据和 `.impeccable/design.json` 的色彩元数据。`--check` 同时校验生成漂移、指定对比度、退役颜色和核心产品代码中的颜色字面量。

规范依据：

- Token 文件遵循 [DTCG Format Module 2025.10](https://www.designtokens.org/TR/2025.10/format/) 的类型和值结构。
- 颜色分层参考 [GitHub Primer](https://www.primer.style/product/getting-started/foundations/color-usage/)、[Fluent 2](https://fluent2.microsoft.design/design-tokens)、[Carbon](https://carbondesignsystem.com/elements/color/overview/) 与 [Ant Design](https://ant.design/docs/spec/colors/) 的产品语义映射。
- 对比度、焦点、键盘操作、目标尺寸和非颜色状态表达以 [WCAG 2.2](https://www.w3.org/TR/WCAG22/) AA 为最低门槛。

## 色彩

### 品牌与壳层

梅紫色阶用于建立品牌关系，不等于所有交互都使用同一个颜色。浅色主操作使用梅紫 700，暗色主操作使用梅紫 400；主操作的悬停和按下状态由组件 token 明确定义。应用壳使用更深的梅紫表面，壳层中的文字、图标和定位器使用专属高对比前景。

Web 仅将侧栏作为大面积品牌色面。页面页头、表单、表格和插件管理 Host 保持中性。Launcher 的标题栏与 76px 导航轨共同构成品牌壳层，主工作区保持中性。

### 语义颜色

- `brand`：品牌前景、主操作、链接和当前选择。
- `attention`：需要人工判断或确认的内容。
- `success`：健康、完成或可用。
- `warning`：风险、降级或需要留意。
- `danger`：失败、阻塞或破坏性操作。

状态不得只靠颜色表达，必须同时提供文字、图标、形状或结构化标签。`attention`、`warning` 与 `danger` 是三个独立角色。

### 表面与边界

Canvas 承载应用背景；Surface 承载连续工作区；Raised 仅用于确实覆盖其他内容的表面。静态边框表面不叠加大阴影，抽屉、菜单、对话框和 Toast 才使用浮层阴影。

结构边界用于分组，控件边界用于输入、选择和交互目标。两者不得互换，以保证非文本控件在亮暗主题中都达到至少 `3:1` 的可辨识度。

## 排版与密度

界面使用现有系统字体栈，不下载或捆绑新字体。字号限定为 `12/13/14/16/18/24/30px`：

- 24px 用于页面标题；30px 只用于少量真实主状态。
- 18px 与 16px 用于分区和面板标题。
- 14px 用于正文与标准控件。
- 13px 用于标签、表头和紧凑状态。
- 12px 用于辅助元数据，不用于关键操作。

日志、路径、标识符和结构化数据使用现有等宽字体栈。说明正文上限为 `72ch`；数据工作区可按任务需要延展。

间距限定为 `4/8/12/16/24/32px`，圆角限定为 `4/6/10/14/999px`。36px 是桌面标准控件高度，粗指针和窄屏交互目标至少为 44px。

## 定位器标识

定位器使用 24×24 视框、中央实心点、四个向内圆角定位框和 2px 描边。标识不包含字母、渐变或吉祥物。

- 中性变体：用于认证、About 和中性工作区。
- 壳层变体：用于深梅紫标题栏、侧栏和当前导航。
- 单色变体：用于系统托盘和只能使用单色的环境。

导航选中态同时使用完整背景、文字和定位器，不使用单条彩色边线。运行状态定位器只出现在 Launcher 的真实服务控制主任务面。

## 组件规则

### 主操作

每个工作流只保留一个视觉主操作。主按钮必须覆盖默认、悬停、焦点、按下、禁用和加载状态。焦点统一为 2px 外轮廓并保留 2px 间距。

### 字段

字段始终显示标签，使用完整控件边界和实色表面。错误同时提供关联文案；禁用状态保持可读；占位文本不承担标签职责。

### 导航

桌面使用持久侧栏或导航轨，窄屏使用现有抽屉或紧凑导航。当前选择使用完整色面、清晰文字和定位器标识。键盘顺序与路由能力在各断点保持一致。

### 状态与提示

Chip 用于状态和筛选，不替代按钮。人工关注提示使用完整边界、明确标题与直接操作。错误、警告和成功信息必须能被辅助技术读取。

### 空态与异常页

异常插图使用定位器线性语言和语义 token，不使用独立第三方调色板。空态说明当前为空的原因、可执行动作和必要前置条件。

## 动效与层级

状态反馈、内容切换、浮层和 Launcher 工作区分别使用 `160/200/220/300ms`。内容位移动效只改变 `opacity` 与 `transform`；控件反馈可以改变颜色、边界和阴影，不通过宽高、间距或定位等布局属性驱动。主题切换、路由切换和元素自身过渡遵守单一动效所有者规则。

认证画布保留单一 Canvas 2D 和单一 `requestAnimationFrame` 循环，使用稀疏定位节点与低对比连接线。资源预算、页面隐藏暂停、粗指针 30fps 上限和指针交互边界保持固定；`prefers-reduced-motion` 下只绘制静态场。

层级固定为 sticky 100、menu 200、drawer 300、modal 400、toast 500、emergency 600。

## 可访问性

WCAG 2.2 AA 是强制门槛：

- 正文和控件文字满足文本对比度要求。
- 控件边界、焦点和状态图形满足非文本对比度要求。
- 所有操作支持键盘，焦点可见且不被遮挡。
- 粗指针目标至少 44px，桌面目标不低于 WCAG 2.2 的 24px 最低要求。
- 支持 `prefers-reduced-motion` 与 `forced-colors`。
- 状态不依赖颜色单独表达。

## 产品边界

Web 继续使用 Ant Design Vue，Launcher 继续使用 Fluent UI。两端共享 token 语义和定位器标识，不共享组件实现。

内置聊天卡片、插件渲染模板和 `NativeTemplatePreviewFrame` 保持独立预览调色板；承载它们的管理面 Host 使用本设计体系。第三方内容和数据驱动颜色必须在 `design/color-literal-allowlist.json` 中说明边界与原因。
