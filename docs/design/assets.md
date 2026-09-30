# 静态资源来源与用途

资源按实际消费入口和可追溯来源维护。字体、图标和图片的生成输入随仓库保留；产物是否可删除取决于运行引用与设计工具用途。

| 资源 | 来源与生成职责 | 消费入口 |
| --- | --- | --- |
| 青瓷认证壁纸 | [`celadon-glass.png`](../../web/src/assets/auth/celadon-glass.png) 的 `impeccable:prompt` PNG 文本块保存生成提示词；无损 WebP 是运行产物 | Web `AuthLayout.vue` 加载 [`celadon-glass.webp`](../../web/src/assets/auth/celadon-glass.webp)；PNG 保留为原始素材 |
| 品牌标识 | 经用户确认的 AI 辅助人物概念图转为贝塞尔矢量，以 [`design/mark.json`](../../design/mark.json) 保存原版曲线、黑白填充、白色细轮廓与透明背景；暗色版整体反转填充与描边，保持几何不变 | Web、Launcher 的 `RayleaMark` 按应用主题使用原版或反色版；Web 的 [`favicon.svg`](../../web/public/favicon.svg) 与 [`favicon-dark.svg`](../../web/public/favicon-dark.svg) 使用同一母版，并跟随 Web 有效主题 |
| Launcher 图标 | [`design/mark.json`](../../design/mark.json) 经 [`generate-launcher-icons.mjs`](../../scripts/generate-launcher-icons.mjs) 以 Skia Canvas 2D 的 `Path2D` 确定性光栅渲染，PNG 内嵌来源元数据；[`assets/manifest.json`](../../launcher/assets/manifest.json) 记录输入、生成器与产物摘要 | 应用 PNG 与 ICO 固定使用原版；[`tray.png`](../../launcher/assets/tray.png) 与 [`tray-dark.png`](../../launcher/assets/tray-dark.png) 按系统托盘主题使用原版或反色版，均保留透明背景；[`Windows 资源生成器`](../../launcher/scripts/generate-windows-resources.mjs) 将逐尺寸独立渲染的 ICO 写入供 Go 编译链接的资源文件 |
| HarmonyOS Sans SC | [`design/fonts/harmonyos-sans-sc/`](../../design/fonts/harmonyos-sans-sc/upstream.json) 保存华为发布包中未经修改的 Regular、Medium、Bold 三个 TTF；[`upstream.json`](../../design/fonts/harmonyos-sans-sc/upstream.json) 记录来源、版本、字重与 SHA-256，同目录 [`LICENSE.txt`](../../design/fonts/harmonyos-sans-sc/LICENSE.txt) 保留《HarmonyOS Sans 字体许可协议》 | Web、Launcher 通过 [`typography.generated.css`](../../design/typography.generated.css) 的 `@font-face` 加载，作为全部界面文字的首选字体 |
| Noto Sans SC | [`result.css`](../../templates/help.menu/assets/fonts/noto-sans-sc/result.css) 记录 Google Fonts v40 来源、字重及 Unicode 分段；同目录 [`OFL.txt`](../../templates/help.menu/assets/fonts/noto-sans-sc/OFL.txt) 保留 SIL OFL 1.1 授权 | 三个内置聊天模板直接导入字体 CSS；Web 模板预览通过服务端模板资源接口加载同一组字体文件 |

HarmonyOS Sans 许可允许随软件分发未经修改的字体副本，禁止修改字体及其任何组件，也禁止单独分发字体。因此界面字体不做子集化、格式转换或重新封装，Web 与 Launcher 各自随包携带三个 TTF（共约 24.7 MB）。`scripts/generate-design-tokens.mjs` 每次运行都核对字体文件与 `upstream.json` 记录的摘要，并把许可协议复制到 Web 与 Launcher public 目录的 `fonts/HarmonyOS-Sans-LICENSE.txt`；`THIRD_PARTY_NOTICES.md` 收录协议全文。

许可要求在软件中显著声明使用了该字体：Web 在“偏好设置 › 外观”中说明并链接协议原文，Launcher 在“关于”页说明。该许可为可撤销授权，发布前确认授权仍然有效。

Noto Sans SC 子集是模板共用源文件，不按单个页面显示的少量字符删减；插件名、用户输入和中文状态文本都可能需要额外字形。

## 模板预览

正式模板由 `template.json` 指定 HTML 与 CSS，管理预览数据来自 `preview.json`；服务端在 `internal/render` 中读取和渲染这些文件。

`templates/status.panel/preview.html` 保留为静态设计预览。排行榜的独立 `preview.html` 没有模板声明、运行入口或设计工具引用，已移除；排行榜的正式 `template.html`、`styles.css`、输入 schema 和预览数据保留。

## 2026-09-29 核对记录

- HarmonyOS Sans SC 三个 TTF 与上游发布包逐字节一致，名称表版本为 1.0，版权方为 Huawei Device Co., Ltd.，嵌入许可位 fsType 为 8（可编辑嵌入）。Web 与 Launcher 构建产物中的 TTF 摘要与 `upstream.json` 一致，许可协议位于产物的 `fonts/` 目录。
- 每个字重覆盖 29,063 个码位，包含 CJK 统一表意文字基本区全部 20,902 字与扩展 A 区全部 6,582 字。
- Web 与 Launcher 界面改用该字体后，Noto Sans SC 的 WOFF2 分段不再进入两者的构建产物。

## 2026-09-10 核对记录

- 共享字体 CSS 引用的 101 个 WOFF2 文件均存在且文件头有效，总计 4,516,508 字节。
- 从 Web 源码、Launcher Renderer 和模板的 TS、Vue、JSON、HTML 文本抽取 1023 个不同中文字符，使用本机 Chromium 浏览器离线载入共享 CSS。DevTools 返回 1023 个自托管 Noto Sans SC 字形，未使用系统字体回退，HTTP(S) 请求为零。
- 认证 PNG 保留生成提示词，Web 实际引用 WebP；Launcher 图标有生成输入、来源元数据和摘要校验入口。

这次字体检查覆盖当前界面与模板样本文字，不代表任意用户文本、全部 Unicode 字符或所有平台都已经验收。新增字形、替换字体或改变子集时，需重新检查字体加载、授权和实际渲染。
