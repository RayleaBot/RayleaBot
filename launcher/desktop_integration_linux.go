//go:build linux

package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func prepareDesktopIntegration(icon []byte) error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	dataHome := os.Getenv("XDG_DATA_HOME")
	if !filepath.IsAbs(dataHome) {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		dataHome = filepath.Join(home, ".local", "share")
	}
	return registerLauncherDesktop(dataHome, executable, icon)
}

func registerLauncherDesktop(dataHome, executable string, icon []byte) error {
	if !filepath.IsAbs(dataHome) || !filepath.IsAbs(executable) {
		return fmt.Errorf("desktop integration requires absolute paths")
	}
	// The Desktop Entry specification forbids '=' in an executable name.
	if strings.ContainsAny(executable, "=\x00") || strings.ContainsRune(dataHome, '\x00') {
		return fmt.Errorf("path cannot be represented in a desktop entry")
	}
	iconPath := filepath.Join(dataHome, "rayleabot", "launcher-icon.png")
	// Exec quoting is applied before Desktop Entry string escaping. No shell is used.
	quotedExecutable := `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, "`", "\\`", "$", `\$`, "%", "%%").Replace(executable) + `"`
	entry := "[Desktop Entry]\nType=Application\nName=RayleaBot 启动器\n" +
		"Exec=" + desktopEntryString(quotedExecutable) + "\n" +
		"Icon=" + desktopEntryString(iconPath) + "\n" +
		"StartupWMClass=" + launcherApplicationID + "\nTerminal=false\nNoDisplay=true\n"
	// Wayland compositors resolve the window's app_id through a matching desktop file.
	// Keep the icon outside the portable bundle so its path survives moving the bundle.
	if err := writeDesktopFile(iconPath, icon); err != nil {
		return err
	}
	return writeDesktopFile(filepath.Join(dataHome, "applications", launcherApplicationID+".desktop"), []byte(entry))
}

func desktopEntryString(value string) string {
	return strings.NewReplacer(`\`, `\\`, "\n", `\n`, "\r", `\r`, "\t", `\t`).Replace(value)
}

func writeDesktopFile(destination string, content []byte) error {
	current, err := os.ReadFile(destination)
	if err == nil && bytes.Equal(current, content) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(destination), ".raylea-desktop-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(content); err != nil {
		file.Close()
		return err
	}
	if err := file.Chmod(0o644); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), destination)
}
