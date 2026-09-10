# 静态资源来源与用途

资源按实际消费入口和可追溯来源维护。字体、图标和图片的生成输入随仓库保留；产物是否可删除取决于运行引用与设计工具用途。

| 资源 | 来源与生成职责 | 消费入口 |
| --- | --- | --- |
| 青瓷认证壁纸 | [`celadon-glass.png`](../../web/src/assets/auth/celadon-glass.png) 的 `impeccable:prompt` PNG 文本块保存生成提示词；无损 WebP 是运行产物 | Web `AuthLayout.vue` 加载 [`celadon-glass.webp`](../../web/src/assets/auth/celadon-glass.webp)；PNG 保留为原始素材 |
| Launcher 图标 | [`design/mark.json`](../../design/mark.json) 与 [`design/tokens.json`](../../design/tokens.json) 经 [`generate-launcher-icons.mjs`](../../scripts/generate-launcher-icons.mjs) 生成；[`assets/manifest.json`](../../launcher/assets/manifest.json) 记录输入、生成器与产物摘要 | Launcher 嵌入应用与托盘 PNG，Windows 可执行文件使用 ICO |
| Noto Sans SC | [`result.css`](../../templates/help.menu/assets/fonts/noto-sans-sc/result.css) 记录 Google Fonts v40 来源、字重及 Unicode 分段；同目录 [`OFL.txt`](../../templates/help.menu/assets/fonts/noto-sans-sc/OFL.txt) 保留 SIL OFL 1.1 授权 | Web、Launcher 通过 [`typography.generated.css`](../../design/typography.generated.css) 导入，三个内置聊天模板直接导入相同字体 CSS |

`scripts/generate-design-tokens.mjs` 将字体授权复制到 Web 与 Launcher 的 public 目录。字体子集是共用源文件，不按单个页面显示的少量字符删减；插件名、用户输入和中文状态文本都可能需要额外字形。

## 模板预览

正式模板由 `template.json` 指定 HTML 与 CSS，管理预览数据来自 `preview.json`；服务端在 `internal/render` 中读取和渲染这些文件。

`templates/status.panel/preview.html` 保留为静态设计预览，在 `design/color-literal-allowlist.json` 中有明确用途。排行榜的独立 `preview.html` 没有模板声明、运行入口或设计工具引用，已移除；排行榜的正式 `template.html`、`styles.css`、输入 schema 和预览数据保留。

## 2026-09-10 核对记录

- 共享字体 CSS 引用的 101 个 WOFF2 文件均存在且文件头有效，总计 4,516,508 字节。
- 从 Web 源码、Launcher Renderer 和模板的 TS、Vue、JSON、HTML 文本抽取 1023 个不同中文字符，使用本机 Chromium 浏览器离线载入共享 CSS。DevTools 返回 1023 个自托管 Noto Sans SC 字形，未使用系统字体回退，HTTP(S) 请求为零。
- 认证 PNG 保留生成提示词，Web 实际引用 WebP；Launcher 图标有生成输入、来源元数据和摘要校验入口。

这次字体检查覆盖当前界面与模板样本文字，不代表任意用户文本、全部 Unicode 字符或所有平台都已经验收。新增字形、替换字体或改变子集时，需重新检查字体加载、授权和实际渲染。
