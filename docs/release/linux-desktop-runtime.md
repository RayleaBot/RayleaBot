# Linux Desktop Runtime

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
