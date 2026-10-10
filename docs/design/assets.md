# 静态资源来源与用途

资源按实际消费入口和可追溯来源维护。字体、图标和图片的生成输入随仓库保留；产物是否可删除取决于运行引用与设计工具用途。

| 资源 | 来源与生成职责 | 消费入口 |
| --- | --- | --- |
| 品牌标识 | 经用户确认的浅色与暗色人物图分别转为贝塞尔矢量，以 [`design/mark.json`](../../design/mark.json) 保存两套路径、显式填充与细轮廓；导出的 [`mark-light.svg`](../../design/mark-light.svg) 和 [`mark-dark.svg`](../../design/mark-dark.svg) 只包含路径，不嵌入位图。浅色使用黑白配色，暗色保留白发并使用独立灰阶配色 | Web、Launcher 的 `RayleaMark` 按应用主题选择对应版本；Web 的 [`favicon.svg`](../../web/public/favicon.svg) 与 [`favicon-dark.svg`](../../web/public/favicon-dark.svg) 来自同一母版，并跟随 Web 有效主题 |
| Web 界面标识 | [`generate-web-brand-assets.mjs`](../../scripts/generate-web-brand-assets.mjs) 从两版纯 SVG 按每个目标尺寸的四倍分辨率绘制，再按预乘 alpha 进行面积采样；[`manifest.generated.json`](../../web/src/assets/brand/manifest.generated.json) 记录 SVG、生成器及 PNG 产物摘要 | Web 的 `RayleaMark` 使用 [`brand-assets.generated.ts`](../../web/src/preferences/brand-assets.generated.ts) 提供的多分辨率图片，由 `srcset` 与明确的显示尺寸选择适合屏幕像素密度的资源；登录主图标为 `128px`，菜单栏顶部为 `40px`，其余入口默认 `32px` |
| Launcher 图标 | [`design/mark.json`](../../design/mark.json) 经 [`generate-launcher-icons.mjs`](../../scripts/generate-launcher-icons.mjs) 以 Skia Canvas 2D 的 `Path2D` 确定性光栅渲染，PNG 内嵌主题与来源元数据；[`assets/manifest.json`](../../launcher/assets/manifest.json) 记录输入、生成器与产物摘要 | 应用 PNG 与 ICO 固定使用浅色版；[`tray.png`](../../launcher/assets/tray.png) 与 [`tray-dark.png`](../../launcher/assets/tray-dark.png) 按系统托盘主题使用浅色版或独立暗色版，均保留透明背景；[`Windows 资源生成器`](../../launcher/scripts/generate-windows-resources.mjs) 将逐尺寸独立渲染的 ICO 写入供 Go 编译链接的资源文件 |
| HarmonyOS Sans SC | [`design/fonts/harmonyos-sans-sc/`](../../design/fonts/harmonyos-sans-sc/upstream.json) 保存华为发布包中未经修改的 Regular、Medium、Bold 三个 TTF；[`upstream.json`](../../design/fonts/harmonyos-sans-sc/upstream.json) 记录来源、版本、字重与 SHA-256，同目录 [`LICENSE.txt`](../../design/fonts/harmonyos-sans-sc/LICENSE.txt) 保留《HarmonyOS Sans 字体许可协议》 | Web、Launcher 通过 [`typography.generated.css`](../../design/typography.generated.css) 的 `@font-face` 加载，作为全部界面文字的首选字体 |
| Noto Sans SC | [`result.css`](../../templates/help.menu/assets/fonts/noto-sans-sc/result.css) 记录 Google Fonts v40 来源、字重及 Unicode 分段；同目录 [`OFL.txt`](../../templates/help.menu/assets/fonts/noto-sans-sc/OFL.txt) 保留 SIL OFL 1.1 授权 | 三个内置聊天模板直接导入字体 CSS；Web 模板预览通过服务端模板资源接口加载同一组字体文件 |

更新人物母版后，先运行 `node scripts/generate-design-tokens.mjs` 导出两版 SVG，再运行 `node scripts/generate-web-brand-assets.mjs` 生成 Web 界面图片。后者复用 Launcher 已有的 `@napi-rs/canvas` 依赖，保留透明背景；`--check` 仅使用 Node.js 检查来源、尺寸和产物摘要，Web 构建自动执行该检查。SVG 和原生图标继续保留各自的矢量生成流程。

HarmonyOS Sans 许可允许随软件分发未经修改的字体副本，禁止修改字体及其任何组件，也禁止单独分发字体。因此界面字体不做子集化、格式转换或重新封装，Web 与 Launcher 各自随包携带三个 TTF（共约 24.7 MB）。`scripts/generate-design-tokens.mjs` 每次运行都核对字体文件与 `upstream.json` 记录的摘要，并把许可协议复制到 Web 与 Launcher public 目录的 `fonts/HarmonyOS-Sans-LICENSE.txt`；`THIRD_PARTY_NOTICES.md` 收录协议全文。

许可要求在软件中显著声明使用了该字体：Web 在“偏好设置 › 外观”、Launcher 在“关于”页的“界面字体”一项列出字体名称；协议全文随安装包的 `THIRD_PARTY_NOTICES.md` 与字体目录中的许可文件分发。该许可为可撤销授权，发布前确认授权仍然有效。

Noto Sans SC 子集是模板共用源文件，不按单个页面显示的少量字符删减；插件名、用户输入和中文状态文本都可能需要额外字形。

## 模板预览

正式模板由 `template.json` 指定 HTML 与 CSS，管理预览数据来自 `preview.json`；服务端在 `internal/render` 中读取和渲染这些文件。

`templates/status.panel/preview.html` 保留为静态设计预览。

## 2026-09-29 核对记录

- HarmonyOS Sans SC 三个 TTF 与上游发布包逐字节一致，名称表版本为 1.0，版权方为 Huawei Device Co., Ltd.，嵌入许可位 fsType 为 8（可编辑嵌入）。Web 与 Launcher 构建产物中的 TTF 摘要与 `upstream.json` 一致，许可协议位于产物的 `fonts/` 目录。
- 每个字重覆盖 29,063 个码位，包含 CJK 统一表意文字基本区全部 20,902 字与扩展 A 区全部 6,582 字。
- Web 与 Launcher 界面改用该字体后，Noto Sans SC 的 WOFF2 分段不再进入两者的构建产物。

这次字体检查覆盖当前界面与模板样本文字，不代表任意用户文本、全部 Unicode 字符或所有平台都已经验收。新增字形、替换字体或改变子集时，需重新检查字体加载、授权和实际渲染。
