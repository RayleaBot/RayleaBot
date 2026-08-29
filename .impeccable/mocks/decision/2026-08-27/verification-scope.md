# 验证范围

## 方向图

- 每张图检查导航与主体内容、是否为完整桌面视口、文字密度、视觉禁区、演示数据标记。
- 对照页必须通过 HTTP 返回实际图片；文件存在或图像生成工具返回成功都不足以证明用户可见。
- palette-check.json 只记录候选实色的数学对比度，不代表图像像素或真实组件合规。
- 方向确认前没有生产 UI 改动，不运行与该产物无关的全工程构建，也不宣称运行验证完成。

## 实现后的验证矩阵

| 表面 | 视口与主题 | 主要行为 |
| --- | --- | --- |
| Web 登录 | Windows 16:9、Android 长屏，亮暗主题 | 输入、校验、错误、键盘焦点、提交、等待 |
| Web 系统状态 | 同上 | 四项状态、事件、就绪检查、诊断、恢复、更新与运维操作 |
| Web 插件管理 | 同上 | 列表、筛选、设置、独立插件页面的宿主边界、保存与错误 |
| Web 日志 | 同上 | 实时数据、历史检索、滚动、溢出、内容选择 |
| Launcher | Windows 原生窗口的实际约束，亮暗主题 | 预检、启动停止、服务状态、打开管理面、窗口和托盘入口 |

手机允许必要纵向滚动；保留可读字号和触控目标，不能用内容裁切换取单屏。

## 动效证据与待测边界

- web/src/components/auth/AuthParticleField.vue:219 对非 fine pointer 使用 1000 / 30 的绘制间隔。它限制的是登录背景 Canvas，不能据此判断整个应用只有 30fps。
- web/src/motion/runtime.ts 统一持有路由与主题 View Transition，并在不支持时使用回退。
- launcher/src/renderer/src/launcherMotion.ts 的工作区切换使用 WAAPI opacity 动画，持续时间为 300ms。动画持续时间不等于帧率。
- 这些是源码事实，没有在本轮进行浏览器帧时间、长任务或真实 Android 设备测量。
- 性能验收需要同设备、同窗口、同数据、同电源条件下的前后记录，区分浏览器模拟与真实手机。

## 内容来源

路由与分组来自 web/src/router/routes/modules/admin.ts 和 web/src/locales/zh-CN/app.ts。首页结构来自 DashboardView.vue、DashboardToolsPanel.vue、DashboardRecoveryCard.vue 与 DashboardUpdateCard.vue。图中数值均为演示数据。
