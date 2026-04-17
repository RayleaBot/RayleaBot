# Electron 桌面壳实现要点

本文档用于维护 RayleaLauncher 的桌面壳，收敛主进程、渲染层和窗口交互的固定实现约束。

## 主进程约束

- 窗口背景使用实色，不再依赖系统材质穿透；亮色主题对应 `#f8fafc`，暗色主题对应 `#0f172a`，由 `nativeTheme.shouldUseDarkColors` 在创建窗口时决定初始值。
- 自定义窗口标题栏时保持 `frame: false`，并在需要时启用 `roundedCorners`。
- Windows 平台不再应用 `setBackgroundMaterial("acrylic")`，统一使用实色渲染层。
- 支持亮/暗双色主题，跟随系统或用户显式切换。

## 渲染层约束

- `html`、`body`、应用根节点使用实色背景，由 CSS 变量 `--app-bg` 驱动，随 `data-theme` 属性在 `light` 与 `dark` 之间切换。
- 面板使用 Fluent UI v9 组件与实色 CSS 表面，不依赖 `backdrop-filter` 或低透明度背景。
- 需要拖拽窗口的区域设置 `-webkit-app-region: drag`，交互控件显式设置 `no-drag`。
- 顶层布局优先使用稳定的 Grid 或 Flex 结构，保证窗口尺寸变化时标题栏、侧栏和主内容区不互相挤压。
- 主题状态通过 `ThemeProvider` 管理，持久化到 `localStorage`，并同步到 `document.documentElement.dataset.theme` 与 `FluentProvider` 的 `theme` 属性。

## 窗口交互约束

- 最小化、最大化和关闭动作通过 IPC 暴露给渲染层，避免在渲染层直接假设平台窗口能力。
- 渲染层维护最大化状态订阅，用于同步窗口控制按钮图标和边角表现。
- 托盘恢复和窗口关闭语义保持一致，避免出现标题栏按钮与托盘行为不一致的情况。

## 视觉约束

- 状态徽标、警告条和诊断卡片使用统一语义色，不为单页临时定义独立颜色体系。
- 实色面板在亮/暗主题下均保留足够文字对比度，确保系统状态、错误信息和路径字段可读。
- 移动或窄窗口宽度下优先保证信息区块纵向堆叠，避免面板横向挤压导致交互控件不可点击。

## 排查清单

- 顶层容器是否使用实色背景而非透明背景。
- 主进程窗口背景是否为与当前主题一致的实色。
- Windows 平台是否未应用 acrylic 材质。
- 标题栏按钮是否都设置为 `no-drag`。
- 最大化状态变化是否能同步到渲染层。
- 面板文本、按钮和状态条在亮/暗实色背景下是否保持足够对比度。
- 主题切换后 `data-theme` 与 `FluentProvider` 的 `theme` 是否同步更新。
