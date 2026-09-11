package desktop

import (
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

type ServiceHost interface {
	DesktopHost
	ChooseDirectory(title, initialDirectory string) (string, error)
	ChooseFile(title, initialDirectory, filterName, filterPattern string) (string, error)
	Minimise()
	ToggleMaximise()
	IsMaximised() bool
	HideWindow()
	SetThemeMode(mode string)
	ResolveExternalServiceStop(confirmed bool)
	HasPendingExternalServiceStop() bool
	Quit()
}

type Service struct {
	mu sync.Mutex

	coordinator   *Coordinator
	host          ServiceHost
	basePath      string
	heartbeat     *UpdateHeartbeatRequest
	heartbeatOnce sync.Once
	closePending  bool
	exiting       bool
}

func NewService(basePath, initialControlToken string, watcherPID int, heartbeat *UpdateHeartbeatRequest, host ServiceHost) *Service {
	basePath = filepath.Clean(basePath)
	return &Service{
		host:        host,
		basePath:    basePath,
		heartbeat:   heartbeat,
		coordinator: NewCoordinator(basePath, initialControlToken, watcherPID, host),
	}
}

func (s *Service) ServiceShutdown() error {
	coordinator, host, err := s.dependencies()
	if err == nil {
		host.ResolveExternalServiceStop(false)
		return coordinator.Shutdown()
	}
	return err
}

func (s *Service) GetPlatform() string {
	platform := runtime.GOOS
	if runtime.GOOS == "windows" {
		platform = "win32"
	}
	architecture := runtime.GOARCH
	switch architecture {
	case "amd64":
		architecture = "x64"
	case "386":
		architecture = "ia32"
	}
	return platform + "-" + architecture
}

func (s *Service) GetSnapshot() (LauncherSnapshot, error) {
	coordinator, _, err := s.dependencies()
	if err != nil {
		return LauncherSnapshot{}, err
	}
	return coordinator.Snapshot(), nil
}

func (s *Service) Initialize() error {
	coordinator, _, err := s.dependencies()
	if err != nil {
		return err
	}
	if err := coordinator.Initialize(); err != nil {
		return err
	}
	s.heartbeatOnce.Do(func() { CompleteUpdateHeartbeat(s.basePath, s.heartbeat, coordinator) })
	return nil
}

func (s *Service) Refresh() error {
	coordinator, _, err := s.dependencies()
	if err != nil {
		return err
	}
	return coordinator.Refresh()
}

func (s *Service) Start() error {
	coordinator, _, err := s.dependencies()
	if err != nil {
		return err
	}
	return coordinator.Start()
}

func (s *Service) Stop() error {
	coordinator, _, err := s.dependencies()
	if err != nil {
		return err
	}
	return coordinator.Stop()
}

func (s *Service) ResetAdmin() error {
	coordinator, _, err := s.dependencies()
	if err != nil {
		return err
	}
	return coordinator.ResetAdmin()
}

func (s *Service) CheckForUpdates() error {
	coordinator, _, err := s.dependencies()
	if err != nil {
		return err
	}
	coordinator.CheckForUpdates()
	return nil
}

func (s *Service) DownloadUpdate() error {
	coordinator, _, err := s.dependencies()
	if err != nil {
		return err
	}
	coordinator.DownloadUpdate()
	return nil
}

func (s *Service) InstallDownloadedUpdate() error {
	coordinator, _, err := s.dependencies()
	if err != nil {
		return err
	}
	if err := coordinator.InstallDownloadedUpdate(); err != nil {
		return err
	}
	if coordinator.Snapshot().Launcher.ReleaseCheck.Status == "installing" {
		s.requestExit()
	}
	return nil
}

func (s *Service) OpenWebUI(targetPath string) error {
	coordinator, _, err := s.dependencies()
	if err != nil {
		return err
	}
	return coordinator.OpenWebUI(targetPath)
}

func (s *Service) OpenReleasePage() error {
	coordinator, _, err := s.dependencies()
	if err != nil {
		return err
	}
	return coordinator.OpenReleasePage()
}

func (s *Service) OpenRepositoryPage() error {
	coordinator, _, err := s.dependencies()
	if err != nil {
		return err
	}
	return coordinator.OpenRepositoryPage()
}

func (s *Service) OpenLogsDirectory() error {
	coordinator, _, err := s.dependencies()
	if err != nil {
		return err
	}
	return coordinator.OpenLogsDirectory()
}

func (s *Service) SaveSettings(settings LauncherSettings) error {
	coordinator, _, err := s.dependencies()
	if err != nil {
		return err
	}
	return coordinator.SaveSettings(settings)
}

func (s *Service) PreviewResolvedSettings(settings LauncherSettings) (LauncherResolvedSettings, error) {
	coordinator, _, err := s.dependencies()
	if err != nil {
		return LauncherResolvedSettings{}, err
	}
	return coordinator.PreviewResolvedSettings(settings)
}

func (s *Service) ChooseInstallationRoot() (*string, error) {
	coordinator, host, err := s.dependencies()
	if err != nil {
		return nil, err
	}
	return optionalSelection(host.ChooseDirectory("选择 RayleaBot 安装目录", coordinator.Snapshot().Launcher.ResolvedSettings.InstallationRoot))
}

func (s *Service) ChooseServerExecutable() (*string, error) {
	coordinator, host, err := s.dependencies()
	if err != nil {
		return nil, err
	}
	current := coordinator.Snapshot().Launcher.ResolvedSettings.ServerExecutablePath
	pattern := "*"
	if runtime.GOOS == "windows" {
		pattern = "*.exe"
	}
	return optionalSelection(host.ChooseFile("选择 raylea-server", filepath.Dir(current), "RayleaBot Server", pattern))
}

func (s *Service) ChooseConfigFile() (*string, error) {
	coordinator, host, err := s.dependencies()
	if err != nil {
		return nil, err
	}
	current := coordinator.Snapshot().Launcher.ResolvedSettings.ConfigPath
	return optionalSelection(host.ChooseFile("选择用户配置", filepath.Dir(current), "YAML 配置", "*.yaml;*.yml"))
}

func (s *Service) ChooseWorkdir() (*string, error) {
	coordinator, host, err := s.dependencies()
	if err != nil {
		return nil, err
	}
	return optionalSelection(host.ChooseDirectory("选择工作目录", coordinator.Snapshot().Launcher.ResolvedSettings.Workdir))
}

func (s *Service) ExitApplication() error {
	s.requestExit()
	return nil
}

func (s *Service) Minimize() error {
	_, host, err := s.dependencies()
	if err == nil {
		host.Minimise()
	}
	return err
}

func (s *Service) Maximize() error {
	_, host, err := s.dependencies()
	if err == nil {
		host.ToggleMaximise()
	}
	return err
}

func (s *Service) Close() error {
	coordinator, host, err := s.dependencies()
	if err != nil {
		return err
	}
	snapshot := coordinator.Snapshot()
	switch snapshot.Launcher.Settings.CloseBehavior {
	case CloseHideToTray:
		host.HideWindow()
	case CloseExitApplication:
		s.requestExit()
	default:
		showConfirmation := false
		s.mu.Lock()
		if !s.closePending && !s.exiting {
			s.closePending = true
			showConfirmation = true
		}
		s.mu.Unlock()
		if showConfirmation {
			host.Emit("launcher:show-exit-confirm", nil)
		}
	}
	return nil
}

func (s *Service) HasPendingCloseConfirm() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closePending
}

func (s *Service) CloseConfirmResponse(response LauncherCloseConfirmResponse) error {
	coordinator, host, err := s.dependencies()
	if err != nil {
		return err
	}
	if response.Action != "hide" && response.Action != "exit" && response.Action != "cancel" {
		return errors.New("关闭确认 action 使用了不支持的值")
	}
	s.mu.Lock()
	s.closePending = false
	s.mu.Unlock()
	if response.SetAsDefault && response.Action != "cancel" {
		settings := coordinator.Snapshot().Launcher.Settings
		if response.Action == "hide" {
			settings.CloseBehavior = CloseHideToTray
		} else {
			settings.CloseBehavior = CloseExitApplication
		}
		if err := coordinator.SaveSettings(settings); err != nil {
			return err
		}
	}
	if response.Action == "hide" {
		host.HideWindow()
	} else if response.Action == "exit" {
		s.requestExit()
	}
	return nil
}

func (s *Service) ExternalStopConfirmResponse(confirmed bool) error {
	_, host, err := s.dependencies()
	if err == nil {
		host.ResolveExternalServiceStop(confirmed)
	}
	return err
}

func (s *Service) HasPendingExternalStopConfirm() (bool, error) {
	_, host, err := s.dependencies()
	if err != nil {
		return false, err
	}
	return host.HasPendingExternalServiceStop(), nil
}

func (s *Service) SetThemeMode(mode string) error {
	if mode != "system" && mode != "light" && mode != "dark" {
		return errors.New("主题模式使用了不支持的值")
	}
	_, host, err := s.dependencies()
	if err == nil {
		host.SetThemeMode(mode)
	}
	return err
}

func (s *Service) IsMaximized() (bool, error) {
	_, host, err := s.dependencies()
	if err != nil {
		return false, err
	}
	return host.IsMaximised(), nil
}

func (s *Service) requestExit() {
	coordinator, host, err := s.dependencies()
	if err != nil {
		return
	}
	s.mu.Lock()
	if s.exiting {
		s.mu.Unlock()
		return
	}
	s.exiting = true
	s.closePending = false
	s.mu.Unlock()
	host.ResolveExternalServiceStop(false)
	go func() {
		if err := coordinator.Shutdown(); err != nil {
			snapshot := coordinator.Snapshot()
			snapshot.Launcher.LastLocalError = err.Error()
			snapshot.Launcher.StatusHint = "服务进程未停止，启动器保持打开。"
			coordinator.publish(snapshot)
			s.mu.Lock()
			s.exiting = false
			s.mu.Unlock()
			return
		}
		host.Quit()
	}()
}

func (s *Service) dependencies() (*Coordinator, ServiceHost, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.coordinator == nil || s.host == nil {
		return nil, nil, errors.New("Wails 桌面宿主尚未配置")
	}
	return s.coordinator, s.host, nil
}

func optionalSelection(value string, err error) (*string, error) {
	if err != nil {
		return nil, err
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	return &value, nil
}
