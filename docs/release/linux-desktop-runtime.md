# Linux Runtime

`linux-x64-full` 与 `linux-x64-server` 都需要 Chromium 系统共享库才能使用图片渲染和插件浏览器会话。托管 Chromium 不包含这些发行版运行库；仅启动 HTTP 服务不要求桌面会话。

## Chromium 与字体

Ubuntu 22.04 / Debian 12：

```bash
sudo apt-get update
sudo apt-get install -y libnss3 libnspr4 libatk1.0-0 libatk-bridge2.0-0 libcups2 libdrm2 libdbus-1-3 libx11-6 libxcomposite1 libxdamage1 libxext6 libxfixes3 libxrandr2 libgbm1 libxcb1 libxkbcommon0 libasound2 libpango-1.0-0 libcairo2 libglib2.0-0 fonts-noto-cjk
```

Ubuntu 24.04 及更新版本 / Debian 13 中，将以上命令里的 `libatk1.0-0`、`libatk-bridge2.0-0`、`libcups2`、`libasound2`、`libglib2.0-0` 分别替换为同名加 `t64` 后缀的包。其他发行版使用对应运行库包；也可安装发行版的 Chromium 包来带入其依赖与安全更新。

在解压根目录运行 `./raylea-server doctor` 检查当前架构的共享库缓存。托管 Chromium 准备完成后，也可运行 `ldd .deps/store/chromium-linux-x64/<版本>/chrome-linux64/chrome` 检查该二进制的完整依赖；含 `not found` 的项目需要补装。自定义 `render.browser_path` 的浏览器可能有额外依赖。中文渲染还需要中文字体。

## Launcher 桌面运行库

`linux-x64-full` 中的 `RayleaLauncher` 使用系统提供的 GTK 3 和 WebKit2GTK 4.1 动态库。这些桌面运行库不包含在压缩包中，必须在启动 Launcher 前安装。

Ubuntu 22.04 与 Debian 12 安装运行库：

```bash
sudo apt-get update
sudo apt-get install -y libgtk-3-0 libwebkit2gtk-4.1-0
```

Ubuntu 24.04 及更新版本与 Debian 13 安装 t64 运行库：

```bash
sudo apt-get update
sudo apt-get install -y libgtk-3-0t64 libwebkit2gtk-4.1-0
```

其他发行版需要安装提供 GTK 3、WebKit2GTK 4.1、libsoup 3 和 GIO Unix 的运行库包；`-dev` / `-devel` 包只用于从源码构建 Launcher。安装后可在解压根目录检查动态库：

```bash
ldd ./RayleaLauncher | grep 'not found'
```

命令没有输出时，Launcher 所需的动态库均可解析。桌面启动还需要可用的图形会话；仅运行 server 时使用 `linux-x64-server`。
