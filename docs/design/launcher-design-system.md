# RayleaBot Launcher Design System

本设计系统服务于 `launcher/` 的 Electron 桌面启动器界面，基于 Fluent 2 设计语言与 Fluent UI React v9 组件库建立克制、清晰、可维护的深色工具风格。

## 设计原则

- 启动器是本地服务壳和 Web 入口，不做大屏展示面板。
- 视觉层级以信息效率优先，避免重复 hero、重边框和深色表面反复嵌套。
- 主操作必须唯一突出；危险操作明确标识；工具操作统一降级。
- 组件样式优先通过 Fluent UI React v9 tokens 和全局 CSS 变量复用，不在页面中散落硬编码颜色与间距。

## 设计决策

### 为什么用 Fluent UI React v9

- 启动器渲染层已迁移至 React 18，与 Web 管理面共用同一套 Fluent 设计语言。
- Fluent UI React v9 提供 `FluentProvider` + `webDarkTheme` 暗色主题，直接映射深色桌面场景。
- 组件涵盖启动器所需的按钮、单选组、输入框、标签、导航，无需第三方 UI 库。

### 全局 CSS 策略

启动器的全局 CSS（`style.css`）定义了背景渐变、布局网格和非 Fluent 元素的样式（如 `.app-shell`、`.shell-sidebar`、`.hero-card`、`.panel`）。Fluent UI React v9 的 CSS-in-JS 机制负责组件级样式，CSS 变量提供主题色板和间距系统的引用点。

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

Fluent UI React v9 通过 `webDarkTheme` 提供暗色主题 tokens。本启动器在此基础上叠加全局 CSS 变量：

```css
:root {
  /* 背景系统 */
  --color-bg-base: #051019;
  --color-bg-surface: rgba(8, 21, 33, 0.74);
  --color-bg-elevated: rgba(12, 29, 42, 0.5);
  --color-bg-muted: rgba(8, 22, 34, 0.64);

  /* 边框 */
  --color-border-subtle: rgba(166, 217, 255, 0.14);
  --color-border-strong: rgba(166, 217, 255, 0.18);

  /* 文字 */
  --color-text-primary: #e6f1ff;
  --color-text-secondary: rgba(220, 235, 255, 0.78);
  --color-text-muted: rgba(180, 205, 228, 0.72);

  /* 语义色 */
  --color-accent: #82d8ff;
  --color-success: #52d68a;
  --color-warning: #ffd485;
  --color-error: #ff7b7b;

  /* 语义面板 */
  --color-warning-surface: rgba(255, 210, 133, 0.2);
  --color-error-surface: rgba(255, 131, 131, 0.2);
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
| `--radius-small` | `16px` | 按钮、输入框 |
| `--radius-medium` | `18px` | 导航项 |
| `--radius-large` | `26px` | 面板、侧边栏卡片 |
| `--radius-xl` | `30px` | Hero 卡片 |

## 组件样式规范

### 按钮

所有按钮统一使用 `<Button>` 组件，样式通过 `className` 指定全局 CSS 类：

```css
.action {
  border: 1px solid var(--color-border-strong);
  border-radius: var(--radius-small);
  background: rgba(13, 32, 48, 0.72);
  color: #e6f5ff;
  padding: 12px 14px;
  transition: transform 160ms ease, background 160ms ease;
}

.action.primary {
  background: linear-gradient(135deg, #56a6e8, #245d95);
}

.action.ghost {
  background: rgba(10, 24, 36, 0.42);
}

.action.danger {
  background: linear-gradient(135deg, #b44d63, #6e2433);
}
```

### 面板

```css
.panel {
  border: 1px solid var(--color-border-subtle);
  background: var(--color-bg-surface);
  backdrop-filter: blur(18px);
  border-radius: var(--radius-large);
  padding: 22px;
  box-shadow: 0 18px 48px rgba(2, 9, 16, 0.35);
}
```

### 导航项

```css
.nav-item {
  border: 0;
  padding: 14px 16px;
  border-radius: var(--radius-medium);
  background: rgba(12, 29, 42, 0.5);
  color: rgba(233, 243, 255, 0.84);
  transition: transform 160ms ease, background 160ms ease, color 160ms ease;
}

.nav-item:hover,
.nav-item.active {
  background: linear-gradient(135deg, rgba(35, 95, 143, 0.88), rgba(18, 48, 78, 0.92));
  color: #f5fbff;
  transform: translateX(4px);
}
```

### 问题横幅

```css
.issue-banner {
  border-radius: 24px;
  padding: 18px 22px;
  display: grid;
  gap: 6px;
}

.issue-banner.warning {
  border-color: rgba(255, 206, 122, 0.3);
}

.issue-banner.error {
  border-color: rgba(255, 120, 120, 0.3);
}
```

### 指标卡

```css
.metric {
  border-radius: 18px;
  background: var(--color-bg-elevated);
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
  border: 1px solid var(--color-border-subtle);
  background: rgba(5, 16, 26, 0.82);
  color: inherit;
  padding: 12px 14px;
  font: inherit;
  width: 100%;
}

.field-inline .field-input input:focus {
  outline: 1px solid rgba(120, 180, 255, 0.5);
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

## 可维护性规则

- Fluent UI React v9 组件的样式优先通过 `className` + 全局 CSS 变量覆盖，而不是内联样式或 `makeStyles`。
- 新增 token 前优先复用既有 CSS 变量。
- 页面内如需特殊样式，应先评估是否能抽为通用 CSS 类，再决定是否局部定义。
