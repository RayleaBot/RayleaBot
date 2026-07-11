---
name: RayleaBot
description: 冷暖精密工作台式的自托管机器人运维界面
colors:
  light-canvas: "#F3F6F7"
  light-surface: "#FAF9F5"
  light-cool-soft: "#E8F6FC"
  light-warm-soft: "#F8ECE7"
  light-text: "#1F272C"
  light-text-muted: "#58656E"
  light-border: "#D8E0E4"
  light-cool-action: "#0B6B8F"
  light-warm-attention: "#A44F32"
  light-success: "#2F7D5C"
  light-warning: "#8A5600"
  light-danger: "#C2414B"
  dark-canvas: "#11181C"
  dark-surface: "#182126"
  dark-surface-raised: "#202C32"
  dark-cool-soft: "#16323F"
  dark-warm-soft: "#34231E"
  dark-text: "#E9F0F2"
  dark-text-muted: "#A7B4BA"
  dark-border: "#314047"
  cool-signature: "#66CCFF"
  warm-signature: "#D97757"
  dark-success: "#67C99B"
  dark-warning: "#F0B95A"
  dark-danger: "#FF8089"
typography:
  display:
    fontFamily: "Inter, PingFang SC, Segoe UI, system-ui, sans-serif"
    fontSize: "24px"
    fontWeight: 700
    lineHeight: 1.25
    letterSpacing: "-0.01em"
  title:
    fontFamily: "Inter, PingFang SC, Segoe UI, system-ui, sans-serif"
    fontSize: "18px"
    fontWeight: 650
    lineHeight: 1.35
    letterSpacing: "normal"
  body:
    fontFamily: "Inter, PingFang SC, Segoe UI, system-ui, sans-serif"
    fontSize: "14px"
    fontWeight: 400
    lineHeight: 1.55
    letterSpacing: "normal"
  label:
    fontFamily: "Inter, PingFang SC, Segoe UI, system-ui, sans-serif"
    fontSize: "13px"
    fontWeight: 600
    lineHeight: 1.4
    letterSpacing: "0.01em"
  mono:
    fontFamily: "Cascadia Mono, Consolas, monospace"
    fontSize: "13px"
    fontWeight: 400
    lineHeight: 1.5
    letterSpacing: "normal"
rounded:
  control: "8px"
  surface: "12px"
  floating: "16px"
  pill: "999px"
spacing:
  xs: "4px"
  sm: "8px"
  md: "12px"
  lg: "16px"
  xl: "24px"
  xxl: "32px"
components:
  button-primary:
    backgroundColor: "{colors.light-cool-action}"
    textColor: "{colors.light-surface}"
    rounded: "{rounded.control}"
    padding: "8px 14px"
    height: "36px"
  button-attention:
    backgroundColor: "{colors.light-warm-attention}"
    textColor: "{colors.light-surface}"
    rounded: "{rounded.control}"
    padding: "8px 14px"
    height: "36px"
  input:
    backgroundColor: "{colors.light-surface}"
    textColor: "{colors.light-text}"
    rounded: "{rounded.control}"
    padding: "8px 12px"
    height: "36px"
    width: "100%"
  navigation-item:
    backgroundColor: "{colors.light-cool-soft}"
    textColor: "{colors.light-cool-action}"
    rounded: "{rounded.control}"
    padding: "8px 12px"
    height: "36px"
  status-chip:
    backgroundColor: "{colors.light-cool-soft}"
    textColor: "{colors.light-cool-action}"
    rounded: "{rounded.pill}"
    padding: "4px 8px"
    height: "24px"
  section-surface:
    backgroundColor: "{colors.light-surface}"
    textColor: "{colors.light-text}"
    rounded: "{rounded.surface}"
    padding: "16px"
    width: "100%"
  attention-callout:
    backgroundColor: "{colors.light-warm-soft}"
    textColor: "{colors.light-warm-attention}"
    rounded: "{rounded.surface}"
    padding: "12px 16px"
    width: "100%"
  data-row:
    backgroundColor: "{colors.light-surface}"
    textColor: "{colors.light-text}"
    rounded: "{rounded.control}"
    padding: "10px 12px"
    width: "100%"
---

# Design System: RayleaBot

## Overview

**Creative North Star: "冷暖精密工作台"**

RayleaBot 面向需要持续判断系统状态的机器人运维者。操作者可能在白天的工作环境中连续编辑配置，也可能在夜间快速处理恢复事项。界面首次显示跟随系统主题，亮色与暗色具有同等完整的信息层级、状态语义和操作能力。

冷色代表系统、自动化、导航与主操作，暖色代表需要人工判断、确认和关注的时刻。两种温度形成稳定的工作语义，而不是装饰性配色。布局保持紧凑、清晰和可预测，熟悉的控件应让操作者直接进入任务。

视觉层级依靠色面、边框、留白和柔和阴影建立。界面拒绝通用的深色科技感仪表盘、组件拼贴和夸张动效，也不通过巨型指标或装饰图表制造专业感。

**Key Characteristics:**

- 冷色系统操作，暖色人工关注
- 跟随系统的完整双主题
- 紧凑但不拥挤的工作区密度
- 柔和层叠与稳定组件状态
- Web、Launcher 和官方插件共享语义，不共享框架

## Colors

色彩采用克制的双温度策略。高彩度锚点只服务于明确语义，大片表面保持低彩度和足够对比度。

`warm-signature` 使用 `#D97757` 作为暖色锚点。

### Primary

- **冰川蓝** (#66CCFF / #0B6B8F): 用于主操作、当前选择、链接、焦点和自动化活动。浅色主题的文字与控件使用深交互变体，签名色用于非正文强调和暗色交互。
- **冷蓝淡面** (#E8F6FC / #16323F): 用于选中背景、系统信息和轻量反馈，不作为装饰性大面积铺色。

### Secondary

- **陶土橙** (#D97757 / #A44F32): 仅用于需要人工判断、确认或优先关注的内容。它不承担普通主操作，也不代替警告或危险语义。
- **暖橙淡面** (#F8ECE7 / #34231E): 用于人工关注提示和确认区域，必须与明确文案共同出现。

### Tertiary

- **成功** (#2F7D5C / #67C99B): 表示已完成、健康或可用。
- **警告** (#8A5600 / #F0B95A): 表示存在风险、降级或需要留意，但不等于人工关注色。
- **危险** (#C2414B / #FF8089): 表示失败、破坏性操作或阻断状态。

### Neutral

- **冷雾与夜幕画布** (#F3F6F7 / #11181C): 承载应用外壳与工作区背景。
- **暖瓷与深灰表面** (#FAF9F5 / #182126 / #202C32): 承载主要内容和浮起区域。
- **石墨与霜白文本** (#1F272C / #E9F0F2): 用于标题、正文与关键数据。
- **岩灰与银灰辅文** (#58656E / #A7B4BA): 用于说明和次要信息，不用于关键状态。
- **雾灰边界** (#D8E0E4 / #314047): 用于分隔、控件轮廓和静态结构。

### Named Rules

**The Temperature Rule.** 冷色表达系统与自动化，暖色表达人工判断。任何使用都必须能回答它属于哪一类任务语义。

**The Accessible Anchor Rule.** `cool-signature` 与 `warm-signature` 不直接承载浅色主题的普通正文，必须使用对应深色交互变体。

**The Semantic Independence Rule.** 暖色关注、警告和危险是三个独立角色，状态必须同时提供文字、图标或结构化标签。

## Typography

**Display Font:** Inter, PingFang SC, Segoe UI, system-ui, sans-serif
**Body Font:** Inter, PingFang SC, Segoe UI, system-ui, sans-serif
**Label/Mono Font:** Cascadia Mono, Consolas, monospace

**Character:** 单一无衬线字体系统保持原生、清晰和高效。等宽字体只用于日志、标识符、路径和结构化数据，不进入普通标签或正文。

### Hierarchy

- **Display** (700, 24px, line-height 1.25): 页面标题和少量顶层状态，不用于指标数字造势。
- **Title** (650, 18px, line-height 1.35): 分区标题、面板标题和关键工作区名称。
- **Body** (400, 14px, line-height 1.55): 正文、表单说明和详情内容，连续说明文本限制在 `72ch`。
- **Label** (600, 13px, line-height 1.4): 控件标签、表头和紧凑状态标题，不默认使用全大写。
- **Mono** (400, 13px, line-height 1.5): 日志、路径、请求标识符和代码片段。

### Named Rules

**The Quiet Hierarchy Rule.** 层级依靠字号、字重和间距建立，不通过展示字体、过度字距或全大写标签制造噪音。

**The Readable Measure Rule.** 说明正文保持在 `65ch` 到 `75ch`，数据表和日志工作区可根据任务需要延展。

## Elevation

RayleaBot 使用柔和层叠。普通内容依靠相邻色面、边框和间距分层；阴影只用于独立面板、浮层和需要明确前后关系的区域。

### Shadow Vocabulary

- **浅色独立面板**（`box-shadow: 0 2px 8px rgba(31, 39, 44, 0.08)`）：用于具有独立任务边界的表面。
- **浅色浮层**（`box-shadow: 0 16px 40px rgba(31, 39, 44, 0.14)`）：用于抽屉、弹出层和确认界面。
- **暗色独立面板**（`box-shadow: 0 2px 10px rgba(0, 0, 0, 0.28)`）：用于暗色主题中的独立任务表面。
- **暗色浮层**（`box-shadow: 0 18px 48px rgba(0, 0, 0, 0.42)`）：用于暗色主题中需要盖过工作区的浮层。

状态反馈使用 `160ms`，内容切换使用 `200ms`，浮层使用 `220ms`，统一采用 `cubic-bezier(0.16, 1, 0.3, 1)`。`prefers-reduced-motion` 下移除非必要运动并保留即时反馈。

认证画布允许使用低对比的抽象粒子网络。粒子数量随视口在 `80–160` 之间调整，以低速运动、生命周期淡入淡出和 `150px` 内连线形成连续环境层；细指针在约 `196px` 的作用范围内平滑追踪有速度上限的排斥目标，离开后缓慢衰减。细指针环境逐帧绘制，粗指针环境限制为 `30fps`。动画仅作用于无语义 Canvas 背景，任务表面保持静止；页面隐藏时停止帧循环，`prefers-reduced-motion` 环境只绘制静态网络。

### Named Rules

**The Structural Shadow Rule.** 阴影只说明层级关系，静态容器不得因为装饰需要获得阴影。

**The No Lift Rule.** 悬停不使用位移或缩放制造漂浮感，反馈只改变色面、边框、文字或阴影强度。

**The Ambient Motion Budget Rule.** 认证粒子网络使用单一 Canvas 2D 与单一 `requestAnimationFrame` 循环，限制像素比、粒子数量、速度和连线距离，不读取布局、不进入 Vue 响应式更新，也不移动任务表面；该规则是认证入口的局部例外，不作为其他页面的默认模式。

## Components

### Buttons

- **Shape:** 控件圆角使用 `control`，桌面高度 `36px`，粗指针或窄屏目标高度 `44px`。
- **Primary:** 冷色主按钮只承载当前工作流的主要提交或继续操作，同一区域保持唯一突出。
- **Attention:** 暖色按钮只用于人工确认或需要明确判断的动作，不与普通主按钮并列竞争。
- **States:** 所有按钮覆盖默认、悬停、焦点、按下、禁用、加载和错误状态，焦点轮廓至少达到 `3:1`。

### Inputs / Fields

- **Style:** 使用完整细边框、实色表面和 `control` 圆角，字段说明紧邻控件。
- **Focus:** 焦点使用冷色交互轮廓，不依赖阴影或占位文案表达当前字段。
- **Error / Disabled:** 错误同时显示语义色、文字和关联关系；禁用状态保持可读，不使用过低透明度。

### Navigation

- 侧栏、页签、面包屑和工作区入口继续使用标准产品模式。
- 当前选择使用冷蓝淡面、冷色文字和完整轮廓，不使用彩色侧边条。
- 窄屏切换为抽屉或紧凑导航，路由、工作区和操作能力保持完整。

### Status Chips

- 状态标签使用语义色、短文本和必要图标，不把颜色作为唯一信息。
- 标签用于状态与筛选，不承担普通按钮的交互职责。

### Section Surfaces

- 只有独立对象、独立操作或可被整体移动的内容才使用有边界表面。
- 同一工作流内的说明、字段和数据组优先使用间距、分隔线和标题，不使用嵌套卡片。
- 数据工作区保持流动宽度，表单主体上限 `960px`，说明文本上限 `72ch`。

### Attention Callouts

- 人工关注提示使用暖色淡面、完整边框、明确标题和直接操作。
- 警告、危险与人工关注分别使用自己的语义角色，不通过彩色侧边条区分。

### Loading and Empty States

- 内容加载使用与最终结构一致的骨架，不在页面中央放置孤立旋转图标。
- 空态解释当前为空的原因、可执行动作和必要前置条件，不重复页面标题。

## Do's and Don'ts

### Do:

- **Do** 让冷色系统操作和暖色人工关注保持稳定、可解释的任务语义。
- **Do** 首次跟随系统主题，并让亮暗主题具有等价的层级、对比度和状态信息。
- **Do** 使用 `4/8/12/16/24/32px` 间距建立紧凑但不拥挤的节奏。
- **Do** 使用标准控件、可见焦点、文字标签和 reduced-motion 支持完成 WCAG 2.2 AA。
- **Do** 只在真实任务边界使用独立表面和柔和阴影。

### Don't:

- **Don't** 使用通用的深色科技感仪表盘、霓虹蓝或发光边框作为默认风格。
- **Don't** 使用强烈渐变、渐变文字、玻璃拟态和无意义装饰。
- **Don't** 使用同尺寸卡片墙、嵌套卡片和组件拼贴式页面。
- **Don't** 使用 hero 指标模板，用巨型数字、辅助统计和渐变强调代替真实任务层级。
- **Don't** 使用抢占注意力、依赖布局或无资源上限地占用主线程的装饰性动效，也不使用悬停位移表达状态。
- **Don't** 依赖颜色单独表达成功、警告、危险或需要人工处理。
- **Don't** 为了风格重造标准控件，或让 Web、Launcher 和插件页面出现互不一致的操作词汇。
