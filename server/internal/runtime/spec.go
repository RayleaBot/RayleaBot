package runtime

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"rayleabot/server/internal/config"
	"rayleabot/server/internal/deps"
	"rayleabot/server/internal/plugins"
)

const (
	codePlatformInvalidRequest  = "platform.invalid_request"
	codePlatformResourceMissing = "platform.resource_missing"
	codePluginInitTimeout       = "plugin.init_timeout"
	codePluginEventTimeout      = "plugin.event_timeout"
	codePluginInternalError     = "plugin.internal_error"
	codePluginNotHandled        = "plugin.not_handled"
	codePluginProtocolViolation = "plugin.protocol_violation"
	codePluginShutdownTimeout   = "plugin.shutdown_timeout"
	codePluginStopping          = "plugin.stopping"
)

type Error struct {
	Code    string
	Message string
	Err     error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Err == nil {
		return fmt.Sprintf("%s: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func errorf(code, message string, err error) *Error {
	return &Error{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

type BotInfo struct {
	ID       string
	Nickname string
}

type InitPayload struct {
	Bot          BotInfo
	Capabilities []string
}

type Spec struct {
	PluginID      string
	Runtime       string
	Command       string
	Args          []string
	Env           []string
	WorkDir       string
	EntryPath     string
	InitTimeout   time.Duration
	InitMaxTotal  time.Duration
	EventTimeout  time.Duration
	ShutdownGrace time.Duration
}

func BuildSpec(snapshot plugins.Snapshot, repoRoot string, runtimeConfig config.RuntimeConfig) (Spec, error) {
	return BuildSpecWithContext(context.Background(), snapshot, repoRoot, runtimeConfig)
}

func BuildSpecWithContext(ctx context.Context, snapshot plugins.Snapshot, repoRoot string, runtimeConfig config.RuntimeConfig) (Spec, error) {
	if snapshot.PluginID == "" {
		return Spec{}, errorf(codePlatformInvalidRequest, "plugin_id is required for runtime startup", nil)
	}
	if !snapshot.Valid {
		return Spec{}, errorf(codePlatformInvalidRequest, "plugin manifest is not eligible for runtime startup", nil)
	}
	if snapshot.DisplayState == "conflict" {
		return Spec{}, errorf(codePlatformInvalidRequest, "plugin manifest is conflicted and cannot be started", nil)
	}
	if snapshot.Runtime == "" || snapshot.Entry == "" || snapshot.ManifestPath == "" {
		return Spec{}, errorf(codePlatformInvalidRequest, "plugin manifest is missing runtime startup fields", nil)
	}

	manifestPath := resolveManifestPath(repoRoot, snapshot.ManifestPath)
	manifestDir := filepath.Dir(manifestPath)

	if filepath.IsAbs(snapshot.Entry) {
		return Spec{}, errorf(codePlatformInvalidRequest, "plugin entry must be relative to the manifest directory", nil)
	}

	entryPath := filepath.Clean(filepath.Join(manifestDir, filepath.FromSlash(snapshot.Entry)))
	if escapesDir(manifestDir, entryPath) {
		return Spec{}, errorf(codePlatformInvalidRequest, "plugin entry must remain inside the plugin directory", nil)
	}
	entryInfo, err := os.Lstat(entryPath)
	if err != nil {
		if os.IsNotExist(err) {
			return Spec{}, errorf(codePlatformResourceMissing, "plugin entry file is missing", err)
		}
		return Spec{}, errorf(codePlatformResourceMissing, "stat plugin entry file", err)
	}
	if entryInfo.Mode()&os.ModeSymlink != 0 {
		linkTarget, err := os.Readlink(entryPath)
		if err != nil {
			return Spec{}, errorf(codePlatformResourceMissing, "resolve plugin entry symlink", err)
		}
		if escapesDir(manifestDir, resolveSymlinkTarget(entryPath, linkTarget)) {
			return Spec{}, errorf(codePlatformInvalidRequest, "plugin entry must remain inside the plugin directory", nil)
		}
	}

	resolvedManifestDir, err := filepath.EvalSymlinks(manifestDir)
	if err != nil {
		if os.IsNotExist(err) {
			return Spec{}, errorf(codePlatformResourceMissing, "plugin manifest directory is missing", err)
		}
		return Spec{}, errorf(codePlatformResourceMissing, "resolve plugin manifest directory", err)
	}

	resolvedEntryPath, err := filepath.EvalSymlinks(entryPath)
	if err != nil {
		if os.IsNotExist(err) {
			return Spec{}, errorf(codePlatformResourceMissing, "plugin entry file is missing", err)
		}
		return Spec{}, errorf(codePlatformResourceMissing, "resolve plugin entry file", err)
	}
	if escapesDir(resolvedManifestDir, resolvedEntryPath) {
		return Spec{}, errorf(codePlatformInvalidRequest, "plugin entry must remain inside the plugin directory", nil)
	}

	entryInfo, err = os.Stat(resolvedEntryPath)
	if err != nil {
		if os.IsNotExist(err) {
			return Spec{}, errorf(codePlatformResourceMissing, "plugin entry file is missing", err)
		}
		return Spec{}, errorf(codePlatformResourceMissing, "stat plugin entry file", err)
	}
	if entryInfo.IsDir() {
		return Spec{}, errorf(codePlatformInvalidRequest, "plugin entry must be a file", nil)
	}

	command, env, err := runtimeCommand(ctx, snapshot.Runtime, repoRoot, manifestDir, runtimeConfig)
	if err != nil {
		if runtimeErr, ok := err.(*Error); ok {
			return Spec{}, runtimeErr
		}
		return Spec{}, errorf(codePlatformResourceMissing, "resolve managed runtime executable", err)
	}

	initTimeout := durationFromSeconds(runtimeConfig.PluginInitTimeoutSeconds, 10)
	initMaxTotal := durationFromSeconds(runtimeConfig.PluginInitMaxTotalSeconds, 300)

	return Spec{
		PluginID:      snapshot.PluginID,
		Runtime:       snapshot.Runtime,
		Command:       command,
		Args:          []string{resolvedEntryPath},
		Env:           env,
		WorkDir:       resolvedManifestDir,
		EntryPath:     resolvedEntryPath,
		InitTimeout:   initTimeout,
		InitMaxTotal:  initMaxTotal,
		EventTimeout:  durationFromSeconds(runtimeConfig.PluginEventTimeoutSeconds, 5),
		ShutdownGrace: durationFromSeconds(runtimeConfig.ShutdownGraceSeconds, 5),
	}, nil
}

func runtimeCommand(ctx context.Context, runtimeName string, repoRoot string, manifestDir string, runtimeConfig config.RuntimeConfig) (string, []string, error) {
	manager := deps.NewManager(repoRoot)
	switch runtimeName {
	case "python":
		if venvPython, ok := pythonVirtualenvExecutable(manifestDir); ok {
			return venvPython, nil, nil
		}
		command, err := manager.ResolveEntrypoint(ctx, "python-runtime", "python")
		if err != nil {
			return "", nil, errorf(codePlatformResourceMissing, "managed Python runtime is not available", err)
		}
		return command, nil, nil
	case "nodejs":
		command, err := manager.ResolveEntrypoint(ctx, "nodejs-runtime", "node")
		if err != nil {
			return "", nil, errorf(codePlatformResourceMissing, "managed Node.js runtime is not available", err)
		}
		env := nodeRuntimeEnvironment(runtimeConfig)
		return command, env, nil
	default:
		return "", nil, errorf(codePlatformInvalidRequest, "plugin runtime is not supported by the minimal runtime manager", nil)
	}
}

func pythonVirtualenvExecutable(manifestDir string) (string, bool) {
	candidates := []string{
		filepath.Join(manifestDir, ".venv", "bin", "python"),
		filepath.Join(manifestDir, ".venv", "bin", "python3"),
		filepath.Join(manifestDir, ".venv", "Scripts", "python.exe"),
	}
	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() {
			return candidate, true
		}
	}
	return "", false
}

func nodeRuntimeEnvironment(runtimeConfig config.RuntimeConfig) []string {
	if runtimeConfig.NodeMaxOldSpaceSizeMB <= 0 {
		return nil
	}
	return []string{fmt.Sprintf("NODE_OPTIONS=--max-old-space-size=%d", runtimeConfig.NodeMaxOldSpaceSizeMB)}
}

func durationFromSeconds(seconds int, fallback int) time.Duration {
	if seconds <= 0 {
		seconds = fallback
	}
	return time.Duration(seconds) * time.Second
}

func resolveManifestPath(repoRoot, manifestPath string) string {
	if filepath.IsAbs(manifestPath) {
		return manifestPath
	}
	if repoRoot == "" {
		return filepath.Clean(filepath.FromSlash(manifestPath))
	}
	return filepath.Join(repoRoot, filepath.FromSlash(manifestPath))
}

func escapesDir(root, path string) bool {
	relativePath, err := filepath.Rel(root, path)
	if err != nil {
		return true
	}
	return relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(filepath.Separator))
}

func resolveSymlinkTarget(entryPath, linkTarget string) string {
	if filepath.IsAbs(linkTarget) {
		return filepath.Clean(linkTarget)
	}
	return filepath.Clean(filepath.Join(filepath.Dir(entryPath), linkTarget))
}
