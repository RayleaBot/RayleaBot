//go:build !linux || !gtk3

package main

import "github.com/wailsapp/wails/v3/pkg/application"

func prepareLauncherWindow(_ *application.WebviewWindow) {}
