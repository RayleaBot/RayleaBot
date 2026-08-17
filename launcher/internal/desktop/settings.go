package desktop

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	closeAsk  = "ask_every_time"
	closeTray = "hide_to_tray"
	closeExit = "exit_application"
)

type SettingsStore struct {
	basePath string
}

func NewSettingsStore(basePath string) *SettingsStore {
	return &SettingsStore{basePath: filepath.Clean(basePath)}
}

func DiscoverBasePath() string {
	workingDirectory, _ := os.Getwd()
	executable, _ := os.Executable()
	return discoverBasePath(os.Getenv("RAYLEA_INSTALL_ROOT"), workingDirectory, executable)
}

func discoverBasePath(override, workingDirectory, executable string) string {
	if override = strings.TrimSpace(override); override != "" {
		return absoluteClean(override)
	}
	executableRoot := ""
	if executable = strings.TrimSpace(executable); executable != "" {
		executableRoot = FindInstallationRoot(filepath.Dir(executable))
		if hasInstallationMarkers(executableRoot) {
			return executableRoot
		}
	}
	if workingDirectory = strings.TrimSpace(workingDirectory); workingDirectory != "" {
		if root := FindInstallationRoot(workingDirectory); hasInstallationMarkers(root) {
			return root
		}
	}
	if executableRoot != "" {
		return executableRoot
	}
	if workingDirectory != "" {
		return FindInstallationRoot(workingDirectory)
	}
	return "."
}

func (s *SettingsStore) Load() (LauncherSettings, error) {
	defaults := LauncherSettings{InstallationRoot: FindInstallationRoot(s.basePath), CloseBehavior: closeAsk}
	settingsPath := settingsFilePath(defaults.InstallationRoot)
	payload, err := os.ReadFile(settingsPath)
	if errors.Is(err, os.ErrNotExist) {
		return defaults, s.Save(defaults)
	}
	if err != nil {
		return defaults, fmt.Errorf("读取启动器设置: %w", err)
	}

	var loaded LauncherSettings
	if err := json.Unmarshal(payload, &loaded); err != nil {
		if saveErr := s.Save(defaults); saveErr != nil {
			return defaults, fmt.Errorf("修复启动器设置: %w", saveErr)
		}
		return defaults, nil
	}
	normalized, err := normalizeSettings(loaded, defaults.InstallationRoot)
	if err != nil {
		if saveErr := s.Save(defaults); saveErr != nil {
			return defaults, fmt.Errorf("修复启动器设置: %w", saveErr)
		}
		return defaults, nil
	}
	if !hasInstallationMarkers(normalized.InstallationRoot) && hasInstallationMarkers(defaults.InstallationRoot) {
		normalized.InstallationRoot = defaults.InstallationRoot
		normalized.AdvancedOverrides = dropOverridesInsideRoot(normalized.AdvancedOverrides, loaded.InstallationRoot)
	}
	if err := s.Save(normalized); err != nil {
		return LauncherSettings{}, err
	}
	return normalized, nil
}

func (s *SettingsStore) Save(settings LauncherSettings) error {
	normalized, err := normalizeSettings(settings, "")
	if err != nil {
		return err
	}
	resolved := ResolveLauncherSettings(normalized)
	normalized.AdvancedOverrides = removeRedundantOverrides(normalized.AdvancedOverrides, resolved)
	payload, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化启动器设置: %w", err)
	}
	return writeFileAtomically(settingsFilePath(FindInstallationRoot(s.basePath)), append(payload, '\n'), 0o600)
}

func FindInstallationRoot(start string) string {
	current, err := filepath.Abs(strings.TrimSpace(start))
	if err != nil || current == "" {
		current, err = os.Getwd()
		if err != nil {
			return filepath.Clean(start)
		}
	}
	fallback := current
	for {
		if hasInstallationMarkers(current) {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			return fallback
		}
		current = parent
	}
}

func ResolveLauncherSettings(settings LauncherSettings) LauncherResolvedSettings {
	root, _ := filepath.Abs(settings.InstallationRoot)
	server := resolveServerExecutable(root)
	configPath := filepath.Join(root, "config", "user.yaml")
	workdir := root
	if overrides := settings.AdvancedOverrides; overrides != nil {
		server = firstNonEmpty(overrides.ServerExecutablePath, server)
		configPath = firstNonEmpty(overrides.ConfigPath, configPath)
		workdir = firstNonEmpty(overrides.Workdir, workdir)
	}
	return LauncherResolvedSettings{
		InstallationRoot:     root,
		ServerExecutablePath: absoluteClean(server),
		ConfigPath:           absoluteClean(configPath),
		Workdir:              absoluteClean(workdir),
	}
}

func normalizeSettings(settings LauncherSettings, fallbackRoot string) (LauncherSettings, error) {
	settings.InstallationRoot = strings.TrimSpace(settings.InstallationRoot)
	if settings.InstallationRoot == "" {
		settings.InstallationRoot = strings.TrimSpace(fallbackRoot)
	}
	if settings.InstallationRoot == "" {
		return LauncherSettings{}, errors.New("installationRoot 不能为空")
	}
	settings.InstallationRoot = absoluteClean(settings.InstallationRoot)
	switch settings.CloseBehavior {
	case "":
		settings.CloseBehavior = closeAsk
	case closeAsk, closeTray, closeExit:
	default:
		return LauncherSettings{}, errors.New("closeBehavior 使用了不支持的值")
	}
	if overrides := settings.AdvancedOverrides; overrides != nil {
		overrides.ServerExecutablePath = optionalAbsolute(overrides.ServerExecutablePath)
		overrides.ConfigPath = optionalAbsolute(overrides.ConfigPath)
		overrides.Workdir = optionalAbsolute(overrides.Workdir)
		if overrides.ServerExecutablePath == "" && overrides.ConfigPath == "" && overrides.Workdir == "" {
			settings.AdvancedOverrides = nil
		}
	}
	return settings, nil
}

func resolveServerExecutable(root string) string {
	name := "raylea-server"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	candidates := []string{
		filepath.Join(root, name),
		filepath.Join(root, "server", "dist", name),
		filepath.Join(root, "server", name),
		filepath.Join(root, "server", "bin", name),
	}
	for _, candidate := range candidates {
		if regularFile(candidate) {
			return candidate
		}
	}
	return candidates[0]
}

func settingsFilePath(root string) string {
	storageRoot := root
	for current := filepath.Clean(root); ; current = filepath.Dir(current) {
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		if filepath.Base(parent) == ".worktrees" {
			storageRoot = filepath.Dir(parent)
			break
		}
	}
	return filepath.Join(storageRoot, "data", "launcher.json")
}

func hasInstallationMarkers(root string) bool {
	development := regularFile(filepath.Join(root, "server", "go.mod")) && regularFile(filepath.Join(root, "launcher", "package.json"))
	release := regularFile(filepath.Join(root, "config", "default.yaml")) && regularFile(filepath.Join(root, ".deps", "manifest.json"))
	return development || release
}

func removeRedundantOverrides(overrides *LauncherAdvancedOverrides, derived LauncherResolvedSettings) *LauncherAdvancedOverrides {
	if overrides == nil {
		return nil
	}
	copy := *overrides
	base := LauncherSettings{InstallationRoot: derived.InstallationRoot, CloseBehavior: closeAsk}
	defaults := ResolveLauncherSettings(base)
	if samePath(copy.ServerExecutablePath, defaults.ServerExecutablePath) {
		copy.ServerExecutablePath = ""
	}
	if samePath(copy.ConfigPath, defaults.ConfigPath) {
		copy.ConfigPath = ""
	}
	if samePath(copy.Workdir, defaults.Workdir) {
		copy.Workdir = ""
	}
	if copy.ServerExecutablePath == "" && copy.ConfigPath == "" && copy.Workdir == "" {
		return nil
	}
	return &copy
}

func dropOverridesInsideRoot(overrides *LauncherAdvancedOverrides, oldRoot string) *LauncherAdvancedOverrides {
	if overrides == nil {
		return nil
	}
	copy := *overrides
	if pathInsideOrEqual(oldRoot, copy.ServerExecutablePath) {
		copy.ServerExecutablePath = ""
	}
	if pathInsideOrEqual(oldRoot, copy.ConfigPath) {
		copy.ConfigPath = ""
	}
	if pathInsideOrEqual(oldRoot, copy.Workdir) {
		copy.Workdir = ""
	}
	if copy.ServerExecutablePath == "" && copy.ConfigPath == "" && copy.Workdir == "" {
		return nil
	}
	return &copy
}

func writeFileAtomically(destination string, payload []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(destination), ".launcher-*.tmp")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(mode); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(payload); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return replaceFile(temporaryName, destination)
}

func replaceFile(source, destination string) error {
	if err := os.Rename(source, destination); err == nil {
		return nil
	}
	if err := os.Remove(destination); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Rename(source, destination)
}

func firstNonEmpty(value, fallback string) string {
	if value = strings.TrimSpace(value); value != "" {
		return value
	}
	return fallback
}

func optionalAbsolute(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return absoluteClean(value)
}

func absoluteClean(value string) string {
	resolved, err := filepath.Abs(strings.TrimSpace(value))
	if err != nil {
		return filepath.Clean(value)
	}
	return resolved
}

func samePath(left, right string) bool {
	if strings.TrimSpace(left) == "" || strings.TrimSpace(right) == "" {
		return false
	}
	left = absoluteClean(left)
	right = absoluteClean(right)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func pathInsideOrEqual(root, candidate string) bool {
	if strings.TrimSpace(root) == "" || strings.TrimSpace(candidate) == "" {
		return false
	}
	relative, err := filepath.Rel(absoluteClean(root), absoluteClean(candidate))
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func regularFile(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode().IsRegular()
}
