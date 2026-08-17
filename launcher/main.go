package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/RayleaBot/RayleaBot/launcher/internal/desktop"
	"github.com/RayleaBot/RayleaBot/launcher/internal/frontend"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/icons"
)

var singleInstanceKey = [32]byte{
	0x52, 0x61, 0x79, 0x6c, 0x65, 0x61, 0x42, 0x6f,
	0x74, 0x2d, 0x57, 0x61, 0x69, 0x6c, 0x73, 0x2d,
	0x4c, 0x61, 0x75, 0x6e, 0x63, 0x68, 0x65, 0x72,
	0x2d, 0x76, 0x33, 0x2d, 0x6c, 0x6f, 0x63, 0x6b,
}

const (
	externalServiceStopConfirmationTimeout = 30 * time.Second
	initialWindowReadyTimeout              = 10 * time.Second
)

type appHost struct {
	app     *application.App
	window  *application.WebviewWindow
	tray    *application.SystemTray
	service *desktop.Service

	allowQuit           atomic.Bool
	trayMu              sync.Mutex
	externalStopMu      sync.Mutex
	externalStopPending chan bool
	externalStopTimeout time.Duration
}

func main() {
	devServerURL, err := frontend.ResolveDevServer(os.Getenv("FRONTEND_DEVSERVER_URL"), frontend.ProductionBuild)
	if err != nil {
		log.Fatal(err)
	}
	if devServerURL == "" {
		_ = os.Unsetenv("FRONTEND_DEVSERVER_URL")
	} else {
		_ = os.Setenv("FRONTEND_DEVSERVER_URL", devServerURL)
	}
	basePath := desktop.DiscoverBasePath()
	heartbeat, heartbeatEnvironmentPresent := desktop.ConsumeUpdateHeartbeatEnvironment()
	if !heartbeatEnvironmentPresent && desktop.LaunchInterruptedUpdateRecovery(basePath, os.Getpid()) {
		return
	}
	assetFS, err := fs.Sub(frontend.Assets, "dist")
	if err != nil {
		log.Fatal(err)
	}
	icon := launcherIcon()
	host := &appHost{}
	service := desktop.NewService(basePath, consumeEnvironment("RAYLEA_LAUNCHER_CONTROL_TOKEN"), consumePIDEnvironment("RAYLEA_DEV_SERVER_WATCHER_PID"), heartbeat, host)
	app := application.New(application.Options{
		Name:        "RayleaLauncher",
		Description: "RayleaBot 桌面启动器",
		Icon:        icon,
		Services:    []application.Service{application.NewService(service)},
		Assets: application.AssetOptions{
			Handler:        application.AssetFileServerFS(assetFS),
			Middleware:     frontend.SecurityMiddleware(devServerURL),
			DisableLogging: true,
		},
		Windows: application.WindowsOptions{
			DisableQuitOnLastWindowClosed: true,
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID:      "local.rayleabot.launcher",
			EncryptionKey: singleInstanceKey,
			OnSecondInstanceLaunch: func(application.SecondInstanceData) {
				if host.window != nil {
					host.restoreWindow()
				}
			},
		},
		ShouldQuit: func() bool {
			if host.allowQuit.Load() {
				return true
			}
			go service.ExitApplication()
			return false
		},
	})

	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:                       "main",
		Title:                      "RayleaBot 启动器",
		Width:                      1380,
		Height:                     920,
		MinWidth:                   760,
		MinHeight:                  560,
		URL:                        "/",
		Frameless:                  true,
		Hidden:                     true,
		InitialPosition:            application.WindowCentered,
		BackgroundType:             application.BackgroundTypeSolid,
		BackgroundColour:           application.NewRGB(246, 243, 245),
		DefaultContextMenuDisabled: true,
		EnableFileDrop:             false,
		Permissions: map[application.PermissionType]application.Permission{
			application.PermissionMicrophone:    application.PermissionDeny,
			application.PermissionCamera:        application.PermissionDeny,
			application.PermissionGeolocation:   application.PermissionDeny,
			application.PermissionNotifications: application.PermissionDeny,
			application.PermissionClipboardRead: application.PermissionDeny,
		},
		Windows: application.WindowsWindow{Theme: application.SystemDefault},
	})
	tray := app.SystemTray.New()
	host.app, host.window, host.tray, host.service = app, window, tray, service
	var showInitialWindow sync.Once
	showWindow := func() {
		showInitialWindow.Do(func() { window.Show() })
	}
	showFallback := time.AfterFunc(initialWindowReadyTimeout, func() {
		application.InvokeAsync(showWindow)
	})
	defer showFallback.Stop()
	window.OnWindowEvent(initialWindowReadyEvent(), func(*application.WindowEvent) {
		showFallback.Stop()
		showWindow()
	})

	if runtime.GOOS == "darwin" {
		tray.SetTemplateIcon(icons.SystrayMacTemplate)
	} else {
		tray.SetIcon(icon)
	}
	tray.SetTooltip("RayleaBot 启动器")
	tray.OnClick(host.toggleWindow)
	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		host.SetTrayState(desktop.TrayMenuState{TrayStatusSummary: "未启动", TrayServiceAction: "start", TrayServiceActionLabel: "启动服务", CanRunTrayServiceAction: true})
		go func() {
			if err := service.Initialize(); err != nil {
				log.Printf("initialize launcher service: %v", err)
			}
		}()
	})

	window.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		if host.allowQuit.Load() {
			return
		}
		event.Cancel()
		go service.Close()
	})
	window.OnWindowEvent(events.Common.WindowMaximise, func(*application.WindowEvent) {
		host.Emit("launcher:maximized-change", true)
	})
	window.OnWindowEvent(events.Common.WindowUnMaximise, func(*application.WindowEvent) {
		host.Emit("launcher:maximized-change", false)
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

func initialWindowReadyEvent() events.WindowEventType {
	switch runtime.GOOS {
	case "darwin":
		return events.Mac.WebViewDidFinishNavigation
	case "linux":
		return events.Linux.WindowLoadFinished
	default:
		return events.Windows.WebViewNavigationCompleted
	}
}

func (h *appHost) Emit(name string, data any) {
	if h.window != nil {
		h.window.EmitEvent(name, data)
	}
}

func (h *appHost) SetTrayState(state desktop.TrayMenuState) {
	h.trayMu.Lock()
	defer h.trayMu.Unlock()
	application.InvokeAsync(func() {
		menu := h.app.NewMenu()
		menu.Add("RayleaBot 启动器").SetEnabled(false)
		menu.Add("状态：" + state.TrayStatusSummary).SetEnabled(false)
		menu.AddSeparator()
		menu.Add("恢复窗口").OnClick(func(*application.Context) { h.restoreWindow() })
		menu.Add("打开管理界面").SetEnabled(state.CanOpenWebUI).OnClick(func(*application.Context) {
			go h.service.OpenWebUI("")
		})
		menu.Add(state.TrayServiceActionLabel).SetEnabled(state.CanRunTrayServiceAction).OnClick(func(*application.Context) {
			if state.TrayServiceAction == "stop" {
				go h.service.Stop()
			} else {
				go h.service.Start()
			}
		})
		menu.Add("日志目录").OnClick(func(*application.Context) { go h.service.OpenLogsDirectory() })
		menu.AddSeparator()
		menu.Add("完全退出").OnClick(func(*application.Context) { go h.service.ExitApplication() })
		h.tray.SetTooltip("RayleaBot 启动器 · " + state.TrayStatusSummary)
		h.tray.SetMenu(menu)
	})
}

func (h *appHost) OpenURL(value string) error { return h.app.Browser.OpenURL(value) }

func (h *appHost) OpenDirectory(path string) error { return h.app.Browser.OpenFile(path) }

func (h *appHost) ConfirmExternalServiceStop() bool {
	response := make(chan bool, 1)
	h.externalStopMu.Lock()
	if h.externalStopPending != nil {
		h.externalStopMu.Unlock()
		return false
	}
	h.externalStopPending = response
	h.externalStopMu.Unlock()
	if h.window != nil {
		application.InvokeAsync(func() {
			h.externalStopMu.Lock()
			pending := h.externalStopPending == response
			h.externalStopMu.Unlock()
			if pending {
				h.restoreWindow()
				h.Emit("launcher:show-external-stop-confirm", nil)
			}
		})
	}
	timeout := h.externalStopTimeout
	if timeout <= 0 {
		timeout = externalServiceStopConfirmationTimeout
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	confirmed := false
	select {
	case confirmed = <-response:
	case <-timer.C:
	}
	h.externalStopMu.Lock()
	if h.externalStopPending == response {
		h.externalStopPending = nil
	}
	h.externalStopMu.Unlock()
	return confirmed
}

func (h *appHost) ResolveExternalServiceStop(confirmed bool) {
	h.externalStopMu.Lock()
	response := h.externalStopPending
	h.externalStopPending = nil
	h.externalStopMu.Unlock()
	if response != nil {
		response <- confirmed
	}
}

func (h *appHost) HasPendingExternalServiceStop() bool {
	h.externalStopMu.Lock()
	defer h.externalStopMu.Unlock()
	return h.externalStopPending != nil
}

func (h *appHost) ChooseDirectory(title, initialDirectory string) (string, error) {
	dialog := h.app.Dialog.OpenFile().SetTitle(title).CanChooseDirectories(true).CanChooseFiles(false).AttachToWindow(h.window)
	if filepath.IsAbs(initialDirectory) {
		dialog.SetDirectory(initialDirectory)
	}
	return dialog.PromptForSingleSelection()
}

func (h *appHost) ChooseFile(title, initialDirectory, filterName, filterPattern string) (string, error) {
	dialog := h.app.Dialog.OpenFile().SetTitle(title).CanChooseDirectories(false).CanChooseFiles(true).AttachToWindow(h.window)
	if filepath.IsAbs(initialDirectory) {
		dialog.SetDirectory(initialDirectory)
	}
	if filterPattern != "" {
		dialog.AddFilter(filterName, filterPattern)
	}
	return dialog.PromptForSingleSelection()
}

func (h *appHost) Minimise() { h.window.Minimise() }

func (h *appHost) ToggleMaximise() { h.window.ToggleMaximise() }

func (h *appHost) IsMaximised() bool { return h.window.IsMaximised() }

func (h *appHost) HideWindow() { h.window.Hide() }

func (h *appHost) SetThemeMode(mode string) {
	dark := mode == "dark" || (mode == "system" && h.app.Env.IsDarkMode())
	background := application.NewRGB(246, 243, 245)
	if dark {
		background = application.NewRGB(21, 17, 20)
	}
	h.window.SetBackgroundColour(background)
}

func (h *appHost) Quit() {
	h.allowQuit.Store(true)
	h.app.Quit()
}

func (h *appHost) restoreWindow() {
	h.window.Show()
	h.window.Restore()
	h.window.Focus()
}

func (h *appHost) toggleWindow() {
	if h.window.IsVisible() {
		h.window.Hide()
		return
	}
	h.restoreWindow()
}

func consumeEnvironment(name string) string {
	value := strings.TrimSpace(os.Getenv(name))
	_ = os.Unsetenv(name)
	return value
}

func consumePIDEnvironment(name string) int {
	value := consumeEnvironment(name)
	pid, err := strconv.Atoi(value)
	if err != nil || pid <= 0 {
		return 0
	}
	return pid
}

func launcherIcon() []byte {
	canvas := image.NewRGBA(image.Rect(0, 0, 64, 64))
	fillRoundedRect(canvas, 5, 5, 59, 59, 13, color.RGBA{104, 72, 224, 255})
	fillRoundedRect(canvas, 14, 17, 50, 47, 10, color.RGBA{248, 247, 255, 255})
	fillCircle(canvas, 25, 31, 4, color.RGBA{70, 48, 160, 255})
	fillCircle(canvas, 39, 31, 4, color.RGBA{70, 48, 160, 255})
	for x := 24; x <= 40; x++ {
		canvas.SetRGBA(x, 40, color.RGBA{104, 72, 224, 255})
		canvas.SetRGBA(x, 41, color.RGBA{104, 72, 224, 255})
	}
	var output bytes.Buffer
	if err := png.Encode(&output, canvas); err != nil {
		panic(fmt.Sprintf("encode launcher icon: %v", err))
	}
	return output.Bytes()
}

func fillRoundedRect(target *image.RGBA, left, top, right, bottom, radius int, fill color.RGBA) {
	for y := top; y < bottom; y++ {
		for x := left; x < right; x++ {
			dx := 0
			if x < left+radius {
				dx = left + radius - x
			} else if x >= right-radius {
				dx = x - (right - radius - 1)
			}
			dy := 0
			if y < top+radius {
				dy = top + radius - y
			} else if y >= bottom-radius {
				dy = y - (bottom - radius - 1)
			}
			if dx == 0 || dy == 0 || dx*dx+dy*dy <= radius*radius {
				target.SetRGBA(x, y, fill)
			}
		}
	}
}

func fillCircle(target *image.RGBA, centerX, centerY, radius int, fill color.RGBA) {
	for y := centerY - radius; y <= centerY+radius; y++ {
		for x := centerX - radius; x <= centerX+radius; x++ {
			if (x-centerX)*(x-centerX)+(y-centerY)*(y-centerY) <= radius*radius {
				target.SetRGBA(x, y, fill)
			}
		}
	}
}
