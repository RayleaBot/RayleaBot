# RayleaBot Web Management UI

本规范定义 `web/` 管理面的视觉与交互映射。项目级颜色、字体、层次和组件规则以根目录 [`DESIGN.md`](../../DESIGN.md) 为准；工程栈与请求边界以 [`web-admin-baseline.md`](../engineering/web-admin-baseline.md) 和 `web/AGENTS.md` 为准。

## 工程边界

- 组件继续使用 Ant Design Vue，页面壳与工作区行为继续按 Vue Vben Admin 对齐方案组织。
- HTTP、WebSocket、Pinia store、generated types 和路由语义保持现有正式来源。
- 项目级语义通过 Ant Design theme tokens 与现有 CSS variables 映射，不建立第二套组件库或运行时主题服务。
- 页面局部样式只负责业务布局和无法由组件 token 表达的最小差异。

## 主题映射

主题偏好包含 `system`、`light` 和 `dark`。首次显示使用 `system`，显式选择保存在本地偏好中；亮暗主题提供相同的内容、状态和操作能力。

| 产品语义 | Ant Design / Web 角色 | 浅色 token | 暗色 token |
| --- | --- | --- | --- |
| 工作区画布 | `colorBgLayout`、页面背景 | `light-canvas` | `dark-canvas` |
| 内容表面 | `colorBgContainer`、卡片与抽屉 | `light-surface` | `dark-surface` |
| 抬升表面 | 弹出层、抽屉、浮动工具 | `light-surface` | `dark-surface-raised` |
| 主文本 | `colorText` | `light-text` | `dark-text` |
| 次文本 | `colorTextSecondary` | `light-text-muted` | `dark-text-muted` |
| 结构边界 | `colorBorder`、`colorSplit` | `light-border` | `dark-border` |
| 主操作 | `colorPrimary`、链接、焦点 | `light-cool-action` | `cool-signature` |
| 人工关注 | 页面级 attention token | `light-warm-attention` | `warm-signature` |
| 成功、警告、危险 | Ant Design semantic tokens | `light-success`、`light-warning`、`light-danger` | `dark-success`、`dark-warning`、`dark-danger` |

人工关注色不映射为 `colorWarning`。需要人工确认的区域使用独立 token、明确标题和直接操作，警告仍使用正式语义色。

## 应用壳

- 侧栏负责稳定的一级与二级导航，当前项使用冷蓝淡面、冷色文字和完整轮廓，不使用彩色侧边条。
- 页头承载面包屑、搜索、主题和会话操作，保持单行优先，不与页面主操作竞争。
- 工作区页签只表达稳定页面实例；query 变化继续复用既有 `viewKey` 规则。
- 页面主操作位于页面头或对应工作区的稳定位置，同一区域保持一个冷色主按钮。
- 暖色操作只出现在人工确认、可信代码确认或需要明确判断的上下文中。

## 页面组合

- 数据工作区使用可扩展宽度，表格、日志和模板预览不放入多层卡片。
- 表单主体最大宽度为 `960px`，字段说明紧邻控件，连续说明文本最大宽度为 `72ch`。
- 独立对象、独立操作或需要整体移动的内容可以使用有边界表面；同一工作流内的字段组使用标题、间距和分隔线。
- 状态总览优先使用紧凑行、分区和状态标签，不使用巨型数字加辅助统计的 hero 指标模板。
- 错误、警告和恢复事项使用完整边框、语义淡面、标题、影响和操作，不使用彩色侧边条。

## 组件状态

- 按钮、输入、选择器、页签、导航、表格行和可点击表面覆盖默认、悬停、焦点、按下、禁用、加载和错误状态。
- 桌面控件默认高度 `36px`；窄屏或粗指针环境使用至少 `44px` 的可点击目标。
- 焦点轮廓使用冷色交互 token，轮廓与相邻背景至少达到 `3:1`。
- 加载状态使用与最终内容形状一致的骨架；空态说明原因、前置条件和可执行动作。
- 状态标签同时包含文字或图标，颜色不作为唯一信息。

## 响应式结构

| 视口 | 结构 |
| --- | --- |
| `>= 1200px` | 完整侧栏、页头与多列工作区；多列只用于真实并行任务 |
| `900px - 1199px` | 收紧页边距与辅助栏，主工作区保持完整 |
| `640px - 899px` | 侧栏进入抽屉或紧凑模式，多列工作区降为单列 |
| `< 640px` | 操作按任务优先级换行，表格使用滚动或摘要行，控件目标提升到 `44px` |

响应式变化只调整结构、密度和导航呈现，不使用流式标题字号，也不删除正式功能。

## 验收条件

- 首次主题跟随系统，显式亮暗选择可持久化，切换后无不可读或缺失状态。
- 主文本、次文本、链接、按钮、焦点和语义状态达到 WCAG 2.2 AA。
- 页面不存在嵌套卡片、同尺寸卡片墙、彩色侧边条、渐变文字或装饰性悬停位移。
- 页面继续复用 Ant Design Vue、现有请求层、WebSocket 封装、Pinia stores 和管理深链 helper。
- `prefers-reduced-motion` 下移除非必要过渡，状态变化仍能立即理解。
