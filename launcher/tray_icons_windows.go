package main

import "github.com/wailsapp/wails/v3/pkg/w32"

func nativeTrayIconSize() int {
	// 与 Wails 创建 HICON 时使用的尺寸一致，避免将 32px PNG 再缩小。
	return w32.GetSystemMetrics(w32.SM_CXSMICON)
}
