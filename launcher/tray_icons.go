package main

import (
	"embed"
	"fmt"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed assets/tray/*.png
var trayIconVariants embed.FS

func trayIconForSize(size int) []byte {
	if size == 32 {
		return trayIcon
	}
	if data, err := trayIconVariants.ReadFile(fmt.Sprintf("assets/tray/%d.png", size)); err == nil {
		return data
	}
	return trayIcon
}

func setLauncherTrayIcons(tray *application.SystemTray) {
	size := nativeTrayIconSize()
	tray.SetIcon(trayIconForSize(size))
}
