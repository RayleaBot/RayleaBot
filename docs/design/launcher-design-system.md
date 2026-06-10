# RayleaBot Launcher Design System

本设计系统服务于 `launcher/` 的 Electron 桌面启动器界面，基于 Fluent 2 设计语言与 Fluent UI React v9 组件库建立现代前端 Web 风格，支持亮/暗双色主题。

## 设计原则

- 启动器是本地服务壳和 Web 入口，不做大屏展示面板。
- 视觉层级以信息效率优先，使用实色表面、清晰边框和足够的留白。
- 主操作必须唯一突出；危险操作明确标识；工具操作统一降级。
- 组件样式优先通过 Fluent UI React v9 tokens 和全局 CSS 变量复用，不在页面中散落硬编码颜色与间距。
- 亮/暗主题切换必须即时生效，所有自定义表面与 Fluent 组件同步响应。

## 设计决策

### 为什么用 Fluent UI React v9

- 启动器渲染层使用 React 18，与 Web 管理面分离但保持现代前端风格对齐。
- Fluent UI React v9 提供 `FluentProvider` + `webLightTheme` / `webDarkTheme`，原生支持双色主题。
- 组件涵盖启动器所需的按钮、单选组、输入框、标签、导航，无需第三方 UI 库。

### 全局 CSS 策略

启动器的全局 CSS（`style.css`）定义了实色背景、布局网格和非 Fluent 元素的样式（如 `.app-shell`、`.shell-sidebar`、`.hero-card`、`.panel`）。Fluent UI React v9 的 CSS-in-JS 机制负责组件级样式，CSS 变量提供主题色板和间距系统的引用点。

主题切换通过 `data-theme="light"` / `data-theme="dark"` 驱动 CSS 变量变更，同时向 `FluentProvider` 传入对应主题对象，保证自定义表面与 Fluent 组件步调一致。

## Fluent UI React v9 组件映射

| UI 场景 | Fluent 组件 | 说明 |
|---|---|---|
| 主操作按钮 | `<Button appearance="primary">` | 启动服务、打开管理界面 |
| 危险操作按钮 | `<Button appearance="primary">` + CSS `.action.danger` | 停止服务 |
| 次级按钮 | `<Button>` | 默认灰阶按钮 |
| 幽灵按钮 | `<Button appearance="subtle">` | 工具栏操作、编辑路径 |
| 关闭策略单选 | `<RadioGroup>` + `<Radio>` | 询问/隐藏到托盘/完全退出 |
| 路径输入框 | `<Input readOnly>` | 只读路径展示 |
| 可编辑路径输入框 | `<Input>` | 编辑状态下可修改 |
| 导航按钮 | `<Button appearance="subtle">` | 侧边栏导航项 |
| 状态徽章 | Fluent `Badge` + CSS | 阻塞项/警告项/正常项计数 |
| 问题横幅 | 自定义 `.issue-banner` CSS + Fluent 语义色 | 错误/警告等级 banner |
| 面板容器 | 自定义 `.panel` CSS | 卡片表面 |
| 日志面板 | 自定义 `.log-surface` CSS | 等宽字体、低反差背景 |

## Tokens

### 颜色

Fluent UI React v9 通过 `webLightTheme` / `webDarkTheme` 提供主题 tokens。本启动器在此基础上叠加全局 CSS 变量，以 `:root`（暗色默认）和 `:root[data-theme="light"]` 区分两套值：

```css
:root {
  --accent: #1677ff;
  --accent-hover: #4096ff;
  --accent-subtle: rgba(22, 119, 255, 0.15);
  --error: #ff4d4f;
  --error-subtle: rgba(255, 77, 79, 0.15);
  --warning: #faad14;
  --warning-subtle: rgba(250, 173, 20, 0.15);
  --success: #52c41a;
  --success-subtle: rgba(82, 196, 26, 0.15);
  --app-bg: #0f172a;
  --surface-bg: #1e293b;
  --surface-bg-subtle: #1a2535;
  --surface-border: #334155;
  --surface-border-strong: #475569;
  --surface-hover: #27354f;
  --text-primary: #f1f5f9;
  --text-secondary: #cbd5e1;
  --text-muted: #94a3b8;
}

:root[data-theme="light"] {
  --accent: #1677ff;
  --accent-hover: #0958d9;
  --accent-subtle: rgba(22, 119, 255, 0.08);
  --error: #ff4d4f;
  --error-subtle: rgba(255, 77, 79, 0.08);
  --warning: #fa8c16;
  --warning-subtle: rgba(250, 140, 22, 0.08);
  --success: #52c41a;
  --success-subtle: rgba(82, 196, 26, 0.08);
  --app-bg: #f8fafc;
  --surface-bg: #ffffff;
  --surface-bg-subtle: #f1f5f9;
  --surface-border: #e2e8f0;
  --surface-border-strong: #cbd5e1;
  --surface-hover: #f1f5f9;
  --text-primary: #0f172a;
  --text-secondary: #475569;
  --text-muted: #64748b;
}
```

### 间距

| Token | 值 | 用途 |
|---|---|---|
| `--spacing-page` | `24px` | 页面区块节奏 |
| `--spacing-section` | `18px` | 卡片之间的默认间距 |
| `--spacing-row` | `12px` | 表单行、定义列表行距 |
| `--spacing-inline` | `8px` | 行内按钮和 badge 间距 |

### 圆角

| Token | 值 | 用途 |
|---|---|---|
| `--radius-small` | `8px` | 按钮、输入框、小卡片 |
| `--radius-medium` | `12px` | 导航项、面板、卡片 |
| `--radius-large` | `12px` | 侧边栏卡片、设置区 |
| `--radius-xl` | `12px` | Hero 卡片 |

## 组件样式规范

### 按钮

所有按钮统一使用 `<Button>` 组件，样式通过 `className` 指定全局 CSS 类：

```css
.action {
  border: 1px solid var(--surface-border);
  border-radius: var(--radius-small);
  background: var(--surface-bg-subtle);
  color: var(--text-primary);
  padding: 12px 14px;
  transition: transform 160ms ease, background 160ms ease;
}

.action.primary {
  background: var(--accent-subtle);
  border-color: var(--accent);
  color: var(--accent);
}

.action.ghost {
  background: transparent;
  border-color: transparent;
  color: var(--text-secondary);
}

.action.danger {
  background: var(--error-subtle);
  border-color: var(--error);
  color: var(--error);
}
```

### 面板

```css
.panel {
  border: 1px solid var(--surface-border);
  background: var(--surface-bg);
  border-radius: var(--radius-medium);
  padding: 22px;
  box-shadow: var(--shadow-card);
}
```

### 导航项

```css
.nav-item {
  border: 1px solid transparent;
  padding: 14px 16px;
  border-radius: var(--radius-medium);
  background: transparent;
  color: var(--text-secondary);
  transition: background-color 180ms ease, border-color 180ms ease, color 180ms ease;
}

.nav-item:hover {
  background: var(--surface-hover);
  color: var(--text-primary);
}

.nav-item.active {
  background: var(--accent-subtle);
  border-color: var(--accent);
  color: var(--text-primary);
  box-shadow: inset 3px 0 0 var(--accent);
}
```

### 问题横幅

```css
.issue-banner {
  border-radius: var(--radius-medium);
  padding: 18px 22px;
  display: grid;
  gap: 6px;
}

.issue-banner.warning {
  border-color: var(--warning);
  background: var(--warning-subtle);
}

.issue-banner.error {
  border-color: var(--error);
  background: var(--error-subtle);
}
```

### 指标卡

```css
.metric {
  border-radius: var(--radius-medium);
  background: var(--surface-bg-subtle);
  border: 1px solid var(--surface-border);
  padding: 18px;
  display: grid;
  gap: 6px;
}

.metric strong {
  font-size: 2rem;
}
```

### 关闭策略单选组

```css
.radio-label {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.close-behavior {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  margin-bottom: 18px;
}
```

### Fluent Input 样式覆盖

Fluent UI React v9 的 `<Input>` 组件在 `.field-inline` 容器中使用时需要样式适配：

```css
.field-inline .field-input input {
  border-radius: var(--radius-small);
  border: 1px solid var(--surface-border);
  background: var(--surface-bg-subtle);
  color: var(--text-primary);
  padding: 12px 14px;
  font: inherit;
  width: 100%;
}

.field-inline .field-input input:focus {
  outline: 1px solid var(--accent);
}

.field-inline .field-input input:read-only {
  opacity: 0.7;
  cursor: default;
}
```

## 响应式策略

```css
@media (max-width: 1080px) {
  .app-shell {
    grid-template-columns: 1fr;
  }

  .hero-card {
    grid-template-columns: 1fr;
  }

  .content-grid {
    grid-template-columns: 1fr;
  }
}
```

## 主题切换

主题偏好存储在渲染层 `localStorage`（`raylea-theme-mode`），支持 `light` / `dark` / `system` 三档：

- `system`：通过 `matchMedia('(prefers-color-scheme: dark)')` 实时响应系统主题变化。
- `light` / `dark`：固定使用对应主题，不跟随系统。

切换时同步更新：
1. `document.documentElement.dataset.theme`
2. `FluentProvider` 的 `theme` 属性（`webLightTheme` / `webDarkTheme` + 品牌色覆盖）
3. Electron 主进程窗口背景色（仅创建时，基于 `nativeTheme.shouldUseDarkColors`）

## 可维护性规则

- Fluent UI React v9 组件的样式优先通过 `className` + 全局 CSS 变量覆盖，而不是内联样式或 `makeStyles`。
- 新增 token 前优先复用既有 CSS 变量。
- 页面内如需特殊样式，应先评估是否能抽为通用 CSS 类，再决定是否局部定义。
- 禁止重新引入 `backdrop-filter`、`acrylic` 材质或低透明度堆叠等玻璃质感实现。
