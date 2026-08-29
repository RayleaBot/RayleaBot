# RayleaBot 视觉重设计简报

## 范围与产品边界

- 范围：登录页、Web 管理面、Windows Launcher。
- Android 通过响应式 Web 操作；不新增原生 Android 客户端。
- 保留现有导航分组、路由、操作路径、首页信息、正式字段与状态语义；必要的交互调整需有明确理由。
- 优先覆盖插件设置、自定义插件页面的管理面宿主、实时与历史日志、系统状态和插件列表。
- 插件独立仓库的页面内容、聊天卡片和渲染模板不在本轮改写范围内。
- 复用现有 Vue / Ant Design Vue、React / Fluent UI、语义 token、路由与状态源；不迁移工程栈。

## 视觉目标

优雅优先，其次是美观与精致。参考成熟高密度管理界面，以精确排版、稳定对齐、清楚层次和细致控件承载真实内容。

- 保持紧凑但不拥挤的页面；没有巨型标题、英雄区、无用说明和装饰性统计。
- 色彩轻盈顺眼，不采用科技蓝；渐变与玻璃仅局部使用。
- 亮暗主题提供等价操作、可读性和状态区分。
- 品牌标识现代、简约、大气，不使用经典机器人；允许在小面积品牌素材中使用赛璐璐色面。
- 排版参考 Claude Design 的阅读舒适度，不假定其字体授权或机械复制网页标题字体。
- 首页不因参考图包含商业图表而新增统计、曲线或数据字段。
- 不用隐藏溢出或压缩字号伪装自适应。手机保留必要纵向滚动，避免整页横向滚动。
- 数据表与日志优先保留可读性；触控目标、键盘焦点、对比度和减少动态效果必须保持可用。

## 动效与性能

- 动效用于状态反馈、选择、层级切换、抽屉和弹层；不在日志与持续读数上循环表演。
- 优先使用合成友好的 opacity / transform；高成本模糊和折射限定范围。
- 亮暗切换、路由与组件动效明确归属，避免叠加多套过渡。
- 用同设备、同视口、同数据的前后记录验证帧间隔、长任务和输入响应；效果图不证明帧率。
- 未在真实 Android 设备验证前，不宣称手机实测通过。

## 已批准视觉方向

柒柒以“按：雾白·青瓷”批准第二张方向图，正式选择见 [approval.json](approval.json)。[pick.png](pick.png) 是本轮视觉方向依据，概念种子为 `8c683bf9`；登录与 Launcher 使用同一视觉语言适配各自任务和窗口结构。

方向候选保留为决策记录：

1. 瓷白 · 淡藤：明亮实色工作区、细腻淡藤选择态、局部透光工具栏、简洁赛璐璐品牌色面。
2. 雾白 · 青瓷：浅色导航、连续状态横条、精确分隔线与低饱和青瓷绿。
3. 石墨 · 暖杏：深石墨导航、暖白工作区和少量暖杏主操作，保持成熟管理面构图。

所有图为 AI 生成的视觉方向图，使用明确标记的演示数据；细节文案和控件以真实实现为准。方向图不构成运行截图、已实现功能或性能验证结果。

### 图像与完整生成提示词

使用内置 imagegen 工具，每个方向独立生成一次。三张图实际尺寸均为 1672 × 941，接近 16:9；没有通过放大冒充 1920 × 1080。

- [瓷白 · 淡藤图像](assigned.png) · [完整提示词](assigned.prompt.txt)
- [雾白 · 青瓷图像](pick.png) · [完整提示词](pick.prompt.txt)
- [石墨 · 暖杏图像](canon.png) · [完整提示词](canon.prompt.txt)

每张 PNG 内嵌完整生成提示词。来源扫描结果为 3 张图、0 项缺失；对比页的三个图像端点均返回 HTTP 200 和 image/png。

### 方向图的已知细节偏差

图像保留了导航分组与首页信息。部分图标大于实际控件需要；青瓷稿中的少量蓝色事件图标不属于正式品牌色。实现采用功能图标、正式语义色与任务对应的按钮优先级。图中的演示数据、面板宽度和局部边框不构成新增字段或逐 UI 截图复刻要求，源码与正式 contract 决定实际内容。

## 确认与验证边界

视觉方向已批准。实现采用近白画布、中性导航、青瓷交互色、共享折叶和自托管 Noto Sans SC 标题。Web 桌面导航为 244px、页头为 60px；Launcher 导航为 184px、原生标题栏为 44px。页面标题为 22px，正文为 14px。

登录背景是静态折叶色面；界面动效用于状态和层级切换，并尊重 reduced-motion。日志保留单一外框，手机保持自然纵向滚动与至少 44px 的关键交互目标。

视觉截图与验证记录位于 [verification-scope.md](verification-scope.md)。Web 截图使用明确标记的合成场景或 contract fixtures；Launcher 截图来自真实 React / Fluent 组件和 fixture bridge，不代表实际原生窗口或 Android 设备验证。桌面浏览器的 rAF 采样只证明登录页空闲 Canvas 绘制工作为零，不能证明真实设备帧率提高。

插件 iframe 内部仍由独立插件产物控制，其字体、蓝色按钮或标题不属于宿主样式验证结果。DESIGN.md 与三份分面规范记录当前实现及该边界。

## 参考依据

- 用户提供的 Zenith、Ant Design Pro、Flux、Apex 四张截图。
- [Zenith](https://dashboardpack.com/live-demo-preview/?livedemo=391511)
- [Apex](https://dashboardpack.com/live-demo-preview/?livedemo=391333)
- [Flux](https://dashboardpack.com/live-demo-preview/?livedemo=391457)
- [Ant Design Pro](https://preview.pro.ant.design)
- [Claude Design](https://claude.com/product/design)
- [Apple Materials](https://developer.apple.com/design/human-interface-guidelines/materials)

## 过程证据

- impeccable context.mjs 已在同一会话读取 PRODUCT.md、DESIGN.md 与现有工程约束。
- 概念种子：8c683bf9，operate，候选序号 3；用户明确的参考、功能边界与审美禁区优先。
- 第二次取样获得远程候选资料；Node 进程退出时报告 libuv 断言，候选输出完整保留，不能将该次命令标记为成功退出。
- 候选概念及淘汰理由保存在 options.json；图像完整生成提示词与输出文件并存。
