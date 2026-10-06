package main

import (
	"log/slog"
	"strconv"
	"strings"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const webViewRuntimeKey = `SOFTWARE\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}`

func validWebViewVersion(value string) bool {
	parts := strings.Split(strings.TrimSpace(value), ".")
	if len(parts) != 4 {
		return false
	}
	nonzero := false
	for _, part := range parts {
		version, err := strconv.ParseUint(part, 10, 32)
		if err != nil {
			return false
		}
		nonzero = nonzero || version > 0
	}
	return nonzero
}

func webViewRuntimeInstalled(read func(registry.Key, uint32) string) bool {
	for _, hive := range []registry.Key{registry.LOCAL_MACHINE, registry.CURRENT_USER} {
		for _, view := range []uint32{registry.WOW64_32KEY, registry.WOW64_64KEY} {
			if validWebViewVersion(read(hive, view)) {
				return true
			}
		}
	}
	return false
}

func ensureWebViewRuntime() bool {
	if webViewRuntimeInstalled(func(hive registry.Key, view uint32) string {
		key, err := registry.OpenKey(hive, webViewRuntimeKey, registry.QUERY_VALUE|view)
		if err != nil {
			return ""
		}
		defer key.Close()
		value, _, _ := key.GetStringValue("pv")
		return value
	}) {
		return true
	}
	slog.Error("缺少 Microsoft Edge WebView2 Runtime", "code", "launcher.webview2_missing")
	message := windows.StringToUTF16Ptr("启动器需要 Microsoft Edge WebView2 Runtime。\n\n请安装 Evergreen Runtime 后重新启动；离线设备请参照包内 WINDOWS-RUNTIME.md。\n\n现在打开微软下载页面？")
	choice, err := windows.MessageBox(0, message, windows.StringToUTF16Ptr("RayleaBot 启动器"), windows.MB_YESNO|windows.MB_ICONWARNING)
	const messageBoxYes = 6
	if err == nil && choice == messageBoxYes {
		_ = windows.ShellExecute(0, windows.StringToUTF16Ptr("open"), windows.StringToUTF16Ptr("https://developer.microsoft.com/microsoft-edge/webview2/"), nil, nil, windows.SW_SHOWNORMAL)
	}
	return false
}
