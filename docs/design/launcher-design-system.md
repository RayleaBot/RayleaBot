# RayleaBot Launcher Design System

本规范服务于 `launcher/` 的 Wails 桌面启动器。项目级视觉语义以根目录 [`DESIGN.md`](../../DESIGN.md) 为准；Launcher 使用 React 19、Fluent UI React v9、Go 桌面宿主和生成的 typed bindings。

## 产品职责

- Launcher 是本机服务壳、环境预检入口和 Web 管理面入口，不复制 Web 的业务页面或服务端状态机。
- 界面优先回答服务是否可用、是否需要人工处理、当前可以执行什么操作。
- 主操作保持唯一突出：正式可用性允许打开 Web 时显示“管理界面”，否则显示现有启动动作。停止保持危险次级操作及现有确认规则；重置与退出沿用既有危险语义，工具操作保持低权重。
- 系统诊断、恢复和运行环境状态直接展示正式服务结果，不从日志文本推断状态。

## 主题与 token 映射

Launcher 支持 `system`、`light` 和 `dark`，首次显示跟随系统，显式选择可持久化，不提供按时钟自动切换配置。`FluentProvider` 与自定义 CSS variables 必须使用同一有效主题，窗口背景、原生控件和自定义表面保持一致。普通画布、文字、边框和选中背景使用灰白或炭灰中性色，青瓷用于品牌链接、主操作和少数选中标记。`design/tokens.json` 是主题 token 的唯一机器值源，`launcher/src/shared/launcher-theme-tokens.generated.ts` 提供生成值，`launcher-theme.ts` 保留既有消费接口。

主题入口使用显式菜单，按“跟随系统、浅色、深色”排列并显示当前单选项。菜单由 Motion 在 `220ms` 内淡入并上移 `5px`，关闭时在 `160ms` 内淡出；选中反馈在退出期间保持可见，弹层消失后，新主题从主题按钮中心以 `420ms` 圆形展开到整个窗口，并把焦点还给触发按钮；跟随系统自动切换时新主题以 `280ms` 淡入。`prefers-reduced-motion` 下立即完成开合与主题切换。

| 产品语义 | Fluent / Launcher 角色 | 浅色 token | 暗色 token |
| --- | --- | --- | --- |
| 窗口画布 | 页面与窗口背景 | `light-canvas` | `dark-canvas` |
| 内容表面 | Fluent surface 与独立面板 | `light-surface` | `dark-surface` |
| 抬升表面 | Dialog、Popover、浮动确认 | `light-surface-raised` | `dark-surface-raised` |
| 主文本 | `colorNeutralForeground1` | `light-text` | `dark-text` |
| 次文本 | `colorNeutralForeground2` | `light-text-muted` | `dark-text-muted` |
| 边界 | `colorNeutralStroke1` | `light-border` | `dark-border` |
| 主操作填充 | `colorBrandBackground`、主按钮 | `light-primary` | `dark-primary` |
| 品牌前景 | 品牌链接和少量选中标记 | `light-brand-foreground` | `dark-brand-foreground` |
| 焦点 | `colorStrokeFocus2` 与全局焦点轮廓 | `light-focus` | `dark-focus` |
| 玻璃材质 | 雾白画布、内容分组填充、主操作着色与玻璃的不透明降级 | 由 `light-canvas`、`light-surface`、`light-primary`、`light-text` 局部派生 | 由 `dark-canvas`、`dark-surface-raised`、`dark-primary`、`dark-text` 局部派生 |
| 品牌填充内容 | `colorNeutralForegroundOnBrand` | `on-brand` | `on-brand` |
| 人工关注 | 本地 attention token | `light-attention` | `dark-attention` |
| 状态 | Fluent semantic colors | 浅色语义 tokens | 暗色语义 tokens |

人工关注色只标记需要操作者判断或确认的事项，不替代 warning、danger 或普通 primary action。

服务状态使用固定色调映射，状态透镜的色面与导航栏状态点共用同一语义：

| 服务状态 | 色调 | 含义 |
| --- | --- | --- |
| `stopped` | neutral | 服务处于普通停止状态 |
| `starting`、`stopping` | info | 启动器正在执行系统操作 |
| `running` | success | 服务可用 |
| `degraded` | warning | 服务可用但能力受限 |
| `setup_required` | running | 按运行中展示；管理界面负责首次初始化流程 |
| `failed` | danger | 服务启动或就绪失败 |

## 桌面壳结构

- 默认窗口为 `1280×720` 逻辑像素，最小窗口为 `960×560`；创建窗口时根据屏幕工作区与最小尺寸约束调整大小。
- 顶部拖动区高度为 `44px` 且透明，露出雾白画布；顶部放置共享黑白人物标识、窗口标题与窗口控制，不放置页面主操作。
- 窗口使用 `184px` 导航列与单一主内容区，宽度不超过 `1100px` 时导航列为 `156px`。导航栏是列内的玻璃面板，导航项由 Fluent Regular 功能图标、可见文字、可访问名称和完整中性选中色面组成。
- 运行状态、环境检查、日志诊断、偏好设置和关于应用保持稳定分区，切换时保留当前任务上下文。
- 主内容区优先使用单列任务流；只有状态与操作真实并行时才使用双列。
- 日志与路径使用等宽字体，保持可选择、可复制和可横向查看。
- 文本选区使用浏览器和操作系统的默认高亮，不通过 `::selection` 覆盖选区背景或文字颜色；控件选中态独立使用组件主题。
- 主要任务区使用明确的分组边界，次级信息通过数据行、分隔线和留白组织。五个工作区的内容分组都是画布上的半透明填充，玻璃只用于侧栏、按钮与状态透镜。
- 环境检查顶部以状态图标、结论和检查计数概括结果；阻塞与警告项始终展开，正常项按系统核心、运行环境和环境特性分组收起，平台、核心版本、安装路径与服务地址列在末尾的环境信息分组。
- 尚未取得检查结果时显示“尚未检查”；发生本地错误且无检查结果时显示“检查结果不可用”及重新检查入口，不给出可以启动的结论。
- 偏好设置的阅读态使用定义行；输入框、路径选择和单选控件只在编辑态出现。
- 日志诊断顶部是服务状态、日志状态和本地端点的概览条；实际异常日志完整展开并优先显示，打开完整日志位于日志分组内，技术快照默认收起并在自身区域横向滚动，页面只有一个纵向滚动区。
- 运行状态页顶部以状态透镜、状态名和说明呈现正式状态，右侧操作按钮排成一行并保持内容宽度，主操作位于最右侧：服务运行时依次为停止、重启与管理界面，未运行时主操作为启动。由其他进程启动的服务不能由 Launcher 重启，重启按钮保持禁用并在按钮下方说明原因；重启进行中按钮行保持运行时的布局。页头只保留页面标题、进行中的操作和刷新入口。运行详情是带功能图标的定义行分组，没有新的异常输出时收为单行摘要；环境问题与运行环境准备位于右侧关注栏，窗口宽度不超过 `1050px` 时移到主列之后。页面不用装饰性指标填充留白。

## Fluent 组件映射

| 场景 | 组件 | 规则 |
| --- | --- | --- |
| 主操作 | `Button appearance="primary"` | 当前工作流保持唯一，使用青瓷主操作语义；工作区使用青瓷着色的玻璃胶囊 |
| 人工确认 | `Button` + attention token | 只用于需要明确判断的动作，不与警告色混用 |
| 危险操作 | `Button` + danger token | 停止、重置和完全退出，必须有明确结果文案；工作区使用危险色文字的中性玻璃胶囊 |
| 次级操作 | `Button` | 使用中性边界和表面，不与主操作竞争；工作区使用中性玻璃胶囊 |
| 工具操作 | `Button appearance="subtle"` | 编辑路径、刷新、复制和导航工具；工作区中的这类按钮使用中性玻璃胶囊 |
| 文本输入 | `Input` | 完整标签、清晰焦点和禁用状态 |
| 单项选择 | `RadioGroup`、`Radio` | 关闭策略和互斥设置 |
| 状态 | `Badge`、`MessageBar` | 同时提供文字、图标或结构化标签 |
| 确认 | `Dialog` | 仅用于破坏性或不可在原位安全完成的决定 |

自定义 CSS 只补充窗口布局、日志表面、玻璃材质和 Fluent token 无法表达的最小业务差异。页面不得重新实现 Fluent 已提供的按钮、输入、单选、Badge 或 Dialog；玻璃按钮是 Fluent `Button` 上的样式层。

功能图标统一使用 Fluent Regular；状态透镜内的状态符号使用 Fluent Filled。界面人物标识用于应用身份，浅色主题使用原版，暗色主题将填充与描边整体反色，保留母版曲线、手势与透明背景；功能图标用于导航、状态与操作，两者不互相替代。

## 密度与层次

- 桌面控件高度为 `36px`，导航与关键操作目标至少为 `40px`；触控或粗指针场景提升到 `44px`。
- 页面间距使用项目级 `4/8/12/16/24/32px` 标尺，导航、字段和面板按任务关系选择不同节奏。
- 圆角标尺为 `4/6/8/12/999px`；标准控件使用 `8px`，任务表面和浮层使用 `12px`。玻璃与内容分组使用局部圆角，见 [Liquid Glass 材质](#liquid-glass-材质)。
- 不透明的静态独立面板使用 `12px` 圆角与 `1px` 边界，不叠加大范围阴影；玻璃以边缘细线、亮边与暗角表达材质，侧栏与状态透镜另以外侧投影表达悬浮，内容分组只用半透明填充与细分隔，Dialog 使用浮层阴影。
- 普通说明与字段不包裹为卡片；状态摘要不使用 hero 指标模板。
- 选中导航使用完整背景、高对比文字和功能图标，不使用内嵌彩色侧边条。
- 工作区页面标题为 `24px`，环境检查结论与关于应用中的应用名为 `20px`；组标题为 `16px`，正文为 `14px`，任务标签、路径和日志为 `13px`。`12px` 用于窗口 chrome 与辅助元数据；运行状态主值使用 `30px`、`600` 字重和 `1.2` 行高。
- 品牌、标题、正文与 Fluent 控件统一使用共享的 HarmonyOS Sans SC，Fluent 的 `fontFamilyBase` 映射到 `--font-sans`。三个未经修改的 TTF 与许可协议随 Launcher 构建打包，“关于”页声明使用了该字体。
- Vite 开发服务器仅对 Launcher 与共享设计目录开放文件访问，界面字体位于 `design/fonts/`；开发与生产均须实际加载声明的界面字体。

## 动效与反馈

- 控件反馈采用 `100–160ms` 短节奏，工作区采用 `180–220ms`，浮层采用 `200–220ms`；当前控件状态为 `160ms`，工作区切换为 `220ms`，主题菜单与 Dialog 进入为 `220ms`、退出为 `160ms`，主题切换为 `420ms`，跟随系统自动切换为 `280ms`。
- 控件、浮层和主题动效使用 `cubic-bezier(0.16, 1, 0.3, 1)`，只表达状态、反馈、载入和内容显隐。
- 动效由 Motion 驱动。工作区使用 `220ms` 可取消的位移动画，采用 `cubic-bezier(0.25, 0.1, 0.25, 1)`，从下方 `8px` 沉降到原位且不改变透明度；状态与页面内容在点击时立即替换，导航在动画期间持续可交互。主题菜单与 Dialog 的进入和退出由 Motion 负责：Dialog 表面在透明度与 `0.96` 缩放间过渡、遮罩只淡化，退出完成后才卸载内容并释放焦点约束；Fluent 组件自带的弹层与 Dialog 动画保持关闭。主题切换由 Motion 驱动 View Transition 新快照的 `clip-path`。连续操作取消旧动画并从当前位置继续，不使用计时器推断完成状态。
- 服务状态变化时，状态透镜底色以 `200ms` 过渡，状态图标在缩放与淡化中替换；启动中与停止中的同步图标每 `1.4s` 匀速旋转一周，表示操作仍在进行。状态名与说明在原位交叉淡化并伴随 `4px` 位移，离场文字对辅助技术隐藏。
- 可展开区域由 Motion 在 `220ms` 内展开高度并淡入内容，收起为 `160ms`；`details` 在收起结束后才关闭，折叠时内容仍保留在文档中，折叠箭头随展开状态旋转。
- 侧栏选中背景是位于图标与文字之下的独立层，点击导航项时从上一项所在位置由 Motion 以弹簧滑到新导航项。起点在点击时读取，不使用共享布局动画：布局投影会在侧栏每次渲染时读取页面滚动位置并强制同步重排。非点击的切换与减少动态效果时直接出现，强制颜色模式下隐藏这一层，由系统高亮色标记当前项。
- View Transition 不可用时立即替换主题，功能与焦点顺序保持完整。同一元素不叠加 View Transition、Motion 和 CSS 动画。
- 悬停不平移或缩放面板；玻璃按钮、主操作、对话框按钮与主题按钮按下时由 Motion 在 `120ms` 内缩小到 `0.96`（对话框选项卡片为 `0.985`），松开后以弹簧回弹，状态仍由色面和边界表达；禁用按钮与减少动态效果时不缩放。日志追加不逐条播放进入动画。
- `prefers-reduced-motion` 或 forced-colors 下关闭非必要动画，进度与忙碌状态保留静态文字或图标。动效时长不代表帧率或性能承诺。
- 减少动态效果分别作用于 Fluent 控件、主题浮层、工作区和进度反馈，不依赖全局极短动画时长覆盖；未知进度保留进行中的文字和无障碍状态。

## Liquid Glass 材质

Launcher 参考 Apple 的 [Liquid Glass](https://developer.apple.com/design/) 与 Human Interface Guidelines 的分层：玻璃只属于浮在内容之上的导航层与控件，内容分组留在内容层。材质由 [`liquid-glass.css`](../../launcher/src/renderer/src/liquid-glass.css) 与 [`glassSurfaces.ts`](../../launcher/src/renderer/src/glassSurfaces.ts) 实现，元素通过 `data-glass` 声明 `clear`、`regular` 或 `prominent` 变体。玻璃的填充、边缘与投影转写自 [Apple Design Resources](https://developer.apple.com/design/resources/) 中 macOS 27 UI Kit 的 Liquid Glass 图层样式，作为 `liquid-glass.css` 的局部变量；画布与内容分组从主题 token 以 `color-mix` 局部派生。两者都不新增共享 token，仓库也不包含套件文件、SF 字体或 SF Symbols。

- 画布与 Web 管理壳共用暖石灰光场：五层柔和径向光晕叠在中性画布上，暗色主题使用炭色画布与更暗的光晕；Go 宿主窗口先绘制同色画布，切换主题时不闪烁。画布不使用图片。
- `regular` 用于侧栏与次级按钮，按尺寸取不同配方：按钮使用套件的 `Regular - Small`，浅色是半透明浅灰胶囊，暗色是略亮于画布的炭灰胶囊，几乎没有外侧投影；侧栏使用 `Regular - Large`，填充更白并带纵向外侧投影。`prominent` 用于主操作：套件把着色玻璃画成平涂色面，Launcher 在青瓷填充上叠加顶部受光的渐变高光、上下亮边、细深色描边和同色投影，使着色部分读作玻璃。
- 边缘由多层内阴影组成：两侧与四周有深色细线，上下边缘有亮线和向内衰减的亮边，两侧向内渐暗。Sketch 样式中的 Lighten、Darken、Luminosity、Plus darker 与 Plus lighter 混合改写为半透明填充和黑白内阴影，按 Launcher 画布算出与原混合一致的明度。
- 侧栏、按钮与主操作只由背景色、渐变与阴影构成，不运行脚本，也不读取背景。
- `clear` 状态透镜完全通透，沿用 `Regular - Medium` 的边缘亮线并带一层淡投影；`Regular - Small` 的两侧暗角会让通透边缘发灰，因此不用于透镜。透镜边缘是圆角斜面：视线在斜面处折射，把背后更靠内的状态色压缩成紧贴边缘的细带，中心不放大，透镜边缘因此形成同色光环。WebView2 中该折射由 `glassSurfaces.ts` 生成的 SVG backdrop filter 完成，同一尺寸的透镜共用一个滤镜；WebKit 与 Gecko 的透镜保持通透、不折射。
- 侧栏与按钮位于平整画布或内容填充之上，折射在这里不可见，因此不使用 backdrop-filter；悬停、滚动、切换工作区和缩放窗口都不触发滤镜重算。
- reduced-transparency 下玻璃填充改为完整不透明表面，状态透镜显示为实色圆面；forced-colors 下去掉玻璃边缘与投影，改用系统颜色与边界。
- 内容分组、日志表面与关注面板是半透明填充，自身不设 backdrop-filter。
- 圆角：侧栏 `18px`，内容分组 `20px`，按钮为胶囊，状态透镜为圆形。玻璃按钮、主操作与主题按钮以 `2px` 外侧焦点轮廓（偏移 `2px`）显示键盘焦点，禁用时整体变淡；导航项与窗口控制使用内侧轮廓。
- 五个工作区共用同一构成：页头与内容直接位于画布上，组标题位于分组之外，分组内用带功能图标的数据行、细分隔和可展开行组织。内容列宽为运行状态与日志诊断 `1120px`、环境检查与偏好设置 `960px`、关于应用 `800px`。

## 浮层材质

- Dialog 是 Launcher 中模糊背后窗口的玻璃面板：沿用 `Regular - Large` 的边缘亮线，叠加顶部高光、`28px` 圆角、固定 `24px` 模糊与 `190%` 饱和度，并带深投影；遮罩只轻度压暗窗口，玻璃后方的内容以模糊色块透出。Fluent 把 Dialog 挂载在 `launcher-theme` 容器之外，玻璃选择器因此使用 Fluent provider 的类名。
- Dialog 标题使用 `48px` 小号状态透镜：重置凭据为危险色，关闭启动器与其他待确认操作为人工关注色。选项与按钮位于玻璃之上，使用半透明白色填充与细亮边，不再叠加玻璃；确认按钮沿用主操作的着色玻璃配方，按危险或人工关注着色。
- 主题菜单沿用套件 `Menus` 的玻璃样式：半透明中性填充、上下亮边、细描边与柔和投影，`16px` 圆角，固定 `24px` 模糊与 `190%` 饱和度。菜单项是 `10px` 圆角的行，悬停的行显示中性高亮，当前主题由青瓷勾选标记表示。开合动画只作用于菜单表面，Fluent 弹层自带的进入动画保持关闭：祖先元素上保留的透明度动画会让模糊只读取弹层内部，看不到背后的窗口。所有浮层的模糊半径固定，不随指针、滚动或动画变化。
- 选中背景和文字使用中性色，玻璃不改变状态或操作语义。
- 不支持 backdrop-filter，或启用 reduced-transparency、forced-colors 时，浮层降为完整不透明表面，Dialog 中的状态透镜显示为实色圆面。

## 原生图标与打包

- [`design/mark.json`](../../design/mark.json) 是黑白人物标识的矢量母版；[`scripts/generate-launcher-icons.mjs`](../../scripts/generate-launcher-icons.mjs) 读取原版路径、黑白填充与白色细轮廓，用 Launcher 工作区安装的 Skia 后端 Canvas 2D 包 `@napi-rs/canvas` 光栅渲染 [`launcher/assets/`](../../launcher/assets/) 中的应用 PNG、亮暗托盘 PNG 与 Windows ICO。暗色托盘版同步反转填充与描边，几何与透明背景保持不变；生成不依赖主题 token、SVG 来源文件、浏览器或 Wails CLI。
- 应用 PNG 为 `1024×1024`，固定使用白发原版；`tray.png` 与 `tray-dark.png` 均为 `32×32`，分别使用原版与用户确认的整体反色版。母版路径经 `Path2D` 直接绘制，产物字节只取决于输入与 `@napi-rs/canvas` 版本。标识来自经用户确认的 AI 辅助人物概念图，再转为贝塞尔矢量；PNG 内嵌对应来源及确定性渲染元数据。
- Windows ICO 包含 `16/24/32/48/64/128/256px` 七种尺寸，每种尺寸按目标像素独立渲染，Windows EXE 与应用文件图标固定使用原版。Go 宿主通过 Wails 的 `SetIcon` 与 `SetDarkModeIcon` 提供两种托盘 PNG，由系统托盘主题选择；Launcher 界面标识则跟随应用的有效主题。
- Windows 资源由 [`generate-windows-resources.mjs`](../../launcher/scripts/generate-windows-resources.mjs) 使用冻结的 Wails `v3.0.0-beta.9` 生成对应架构的 `rsrc_windows_<arch>.syso`；根目录开发启动、Launcher 独立开发与打包均在 Go 编译前调用。缓存核对 ICO、Windows manifest、Go 模块、生成器输入及产物，资源缺失或漂移时重新生成；Windows manifest 使用 `asInvoker` 普通用户权限。
- 更新原生图标后需重新构建并启动 Launcher，由新进程载入窗口、托盘和 EXE 图标资源。
- 在仓库根目录运行 `node scripts/generate-launcher-icons.mjs --check` 校验来源与资产摘要。在 Windows 上运行 `python launcher/scripts/verify-windows-icon-resources.py`，验证默认打包 EXE 中的七尺寸图像负载与源 ICO 逐字节一致；其他产物通过 `--exe <path>` 指定。资源校验与实际窗口、任务栏、托盘显示检查分别承担不同验证职责。

## 小窗口策略

- Launcher 最小窗口为 `960×560` 逻辑像素，所有工作区、Dialog 与窗口控制在该尺寸下保持可操作，界面验证以该尺寸为下限。
- 最小宽度等于 `1920×1080` 屏幕在 100% 缩放下的半屏宽度，窗口可以与其他应用左右分屏；最小高度仍能放进该屏幕在 175% 缩放下约 `570px` 高的工作区，放大显示时窗口不会超出屏幕。
- WebView 不开放页面缩放，任何窗口尺寸都保留左侧玻璃导航与单一主工作区，不切换为顶部导航。
- 标题操作、表单和日志按优先级换行或滚动，不截断关键标识符。
- Renderer 根节点不依赖隐藏溢出裁切内容；横向滚动只出现在日志、技术快照或不可安全换行的标识符区域。
- 最小窗口仍保留启动、停止、预检、设置、恢复入口和打开 Web 管理面的完整能力。

## 验收条件

- `system`、`light` 和 `dark` 三种偏好均同步作用于 `FluentProvider`、CSS variables 和窗口背景。
- 主操作、人工关注、警告和危险具有独立且稳定的语义。
- 界面符合 [`DESIGN.md`](../../DESIGN.md) 的 Do's and Don'ts，不使用装饰性循环动效；侧栏、玻璃按钮、状态透镜、主题菜单和 Dialog 的玻璃均具有不透明降级。
- 键盘顺序、焦点、对比度、状态标签、reduced-motion 和 forced-colors 达到 WCAG 2.2 AA。
- Launcher 继续只负责本机进程、系统集成、更新确认和打开 Web，不复制管理面业务。
