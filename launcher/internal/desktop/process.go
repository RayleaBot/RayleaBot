package desktop

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const maxRecentDiagnostics = 40

type ProcessController struct {
	mu sync.RWMutex

	cmd                 *exec.Cmd
	setupToken          string
	controlToken        string
	stderr              []string
	lastStructuredError string
	logDirectory        string
	releaseSupervisor   func()

	runtimeResources map[string]RuntimePrepareResourceProgress
	runtimeActive    bool
	runtimeKind      string
	runtimeSummary   string
	logMu            sync.Mutex
}

func NewProcessController(initialControlToken string) *ProcessController {
	return &ProcessController{
		controlToken:     strings.TrimSpace(initialControlToken),
		logDirectory:     filepath.Join(currentWorkingDirectory(), "logs"),
		runtimeResources: map[string]RuntimePrepareResourceProgress{},
	}
}

func (p *ProcessController) Start(settings LauncherResolvedSettings) error {
	p.mu.Lock()
	if p.cmd != nil {
		p.mu.Unlock()
		return nil
	}
	setupToken, err := secureToken()
	if err != nil {
		p.mu.Unlock()
		return fmt.Errorf("生成初始设置凭据: %w", err)
	}
	controlToken, err := secureToken()
	if err != nil {
		p.mu.Unlock()
		return fmt.Errorf("生成启动器控制凭据: %w", err)
	}
	p.stderr = nil
	p.lastStructuredError = ""
	p.runtimeResources = map[string]RuntimePrepareResourceProgress{}
	p.runtimeActive = false
	p.runtimeKind = ""
	p.runtimeSummary = ""
	p.setupToken = setupToken
	p.controlToken = controlToken
	p.logDirectory = filepath.Join(settings.Workdir, "logs")
	p.mu.Unlock()

	if err := os.MkdirAll(settings.Workdir, 0o755); err != nil {
		p.clearCredentials()
		return fmt.Errorf("创建工作目录: %w", err)
	}
	if err := os.MkdirAll(p.LogDirectory(), 0o755); err != nil {
		p.clearCredentials()
		return fmt.Errorf("创建日志目录: %w", err)
	}

	command := exec.Command(settings.ServerExecutablePath, "-config", settings.ConfigPath)
	command.Dir = settings.Workdir
	command.Env = replaceEnvironmentValues(os.Environ(), map[string]string{
		"RAYLEA_SETUP_TOKEN":            p.SetupToken(),
		"RAYLEA_LAUNCHER_CONTROL_TOKEN": p.ControlToken(),
	})
	configureChildProcess(command)
	stdout, err := command.StdoutPipe()
	if err != nil {
		p.clearCredentials()
		return err
	}
	stderr, err := command.StderrPipe()
	if err != nil {
		p.clearCredentials()
		return err
	}
	if err := command.Start(); err != nil {
		p.recordDiagnostic(err.Error())
		p.clearCredentials()
		return fmt.Errorf("启动 raylea-server: %w", err)
	}
	releaseSupervisor, err := superviseProcess(command.Process)
	if err != nil {
		_ = command.Process.Kill()
		_ = command.Wait()
		p.recordDiagnostic(err.Error())
		p.clearCredentials()
		return fmt.Errorf("建立服务进程监督: %w", err)
	}

	p.mu.Lock()
	p.cmd = command
	p.releaseSupervisor = releaseSupervisor
	p.mu.Unlock()
	go p.consumeOutput("stdout", stdout)
	go p.consumeOutput("stderr", stderr)
	go p.wait(command)
	return nil
}

func replaceEnvironmentValues(environment []string, replacements map[string]string) []string {
	keys := make([]string, 0, len(replacements))
	for key := range replacements {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	result := make([]string, 0, len(environment)+len(keys))
	for _, entry := range environment {
		name, _, found := strings.Cut(entry, "=")
		if !found {
			result = append(result, entry)
			continue
		}
		replaced := false
		for _, key := range keys {
			if strings.EqualFold(name, key) {
				replaced = true
				break
			}
		}
		if !replaced {
			result = append(result, entry)
		}
	}
	for _, key := range keys {
		result = append(result, key+"="+replacements[key])
	}
	return result
}

func (p *ProcessController) wait(command *exec.Cmd) {
	err := command.Wait()
	var releaseSupervisor func()
	p.mu.Lock()
	if p.cmd == command {
		p.cmd = nil
		p.setupToken = ""
		p.controlToken = ""
		releaseSupervisor = p.releaseSupervisor
		p.releaseSupervisor = nil
	}
	p.mu.Unlock()
	if releaseSupervisor != nil {
		releaseSupervisor()
	}
	if err != nil {
		detail := "服务进程异常退出"
		if reason := processExitReason(command.ProcessState); reason != "" {
			detail += "（" + reason + "）"
		}
		if raw := sanitizeDiagnostic(err.Error()); raw != "" {
			detail += "：" + raw
		}
		p.mu.RLock()
		lastStructuredError := p.lastStructuredError
		p.mu.RUnlock()
		if lastStructuredError != "" {
			detail += "。最近错误：" + lastStructuredError
		} else {
			detail += "。未捕获到结构化错误，请打开完整日志查看退出前输出"
		}
		p.recordDiagnostic(detail)
	}
}

func (p *ProcessController) consumeOutput(stream string, reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64<<10), 2<<20)
	for scanner.Scan() {
		line := scanner.Text()
		p.appendLog("server", stream, line+"\n")
		if stream == "stderr" || plainTextStartupDiagnostic(line) {
			p.recordDiagnostic(line)
		}
		p.recordStructuredOutput(line)
	}
	if err := scanner.Err(); err != nil {
		p.recordDiagnostic("读取服务输出失败：" + err.Error())
	}
}

func plainTextStartupDiagnostic(line string) bool {
	normalized := strings.ToLower(strings.TrimSpace(line))
	for _, marker := range []string{"address already in use", "listen tcp", "bind:", "panic:", "fatal error:"} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}

func (p *ProcessController) recordStructuredOutput(line string) {
	var payload map[string]any
	if json.Unmarshal([]byte(line), &payload) != nil {
		return
	}
	level := strings.ToUpper(stringField(payload, "level"))
	if level == "ERROR" || level == "FATAL" {
		message := firstNonEmpty(stringField(payload, "msg"), "服务进程报告错误")
		if cause := stringField(payload, "err"); cause != "" && cause != message {
			message += "：" + cause
		}
		metadata := make([]string, 0, 4)
		for _, item := range []struct{ label, key string }{
			{label: "时间", key: "ts"},
			{label: "错误代码", key: "error_code"},
			{label: "组件", key: "component"},
			{label: "请求 ID", key: "request_id"},
		} {
			if value := stringField(payload, item.key); value != "" {
				metadata = append(metadata, item.label+"："+value)
			}
		}
		if len(metadata) > 0 {
			message += "（" + strings.Join(metadata, "；") + "）"
		}
		p.mu.Lock()
		p.lastStructuredError = message
		p.mu.Unlock()
		p.recordDiagnostic(message)
	}
	if stringField(payload, "component") != "runtime_prepare" && stringField(payload, "msg") != "runtime_prepare_progress" {
		return
	}
	kind := stringField(payload, "resource_kind")
	if kind == "" {
		return
	}
	status := stringField(payload, "status")
	if status != "pending" && status != "running" && status != "succeeded" && status != "failed" {
		status = "running"
	}
	progress := floatPointer(payload["progress"])
	if progress != nil {
		if *progress < 0 {
			*progress = 0
		}
		if *progress > 100 {
			*progress = 100
		}
	}
	label := firstNonEmpty(stringField(payload, "label"), kind)
	item := RuntimePrepareResourceProgress{
		Kind: kind, Label: label, ResourceID: stringField(payload, "resource_id"), Version: stringField(payload, "version"),
		SourceLabel: stringField(payload, "source_label"), SourceURL: stringField(payload, "source_url"), ArchivePath: stringField(payload, "archive_path"), StoreRoot: stringField(payload, "store_root"),
		Stage: firstNonEmpty(stringField(payload, "stage"), "inspect"), Status: status, Progress: progress,
		DownloadedBytes: intPointer(payload["downloaded_bytes"]), TotalBytes: intPointer(payload["total_bytes"]), ExtractedEntries: intPointer(payload["extracted_entries"]), TotalEntries: intPointer(payload["total_entries"]),
		Summary: firstNonEmpty(stringField(payload, "summary"), label+"准备中"), Error: stringField(payload, "err"), UpdatedAt: firstNonEmpty(stringField(payload, "ts"), time.Now().UTC().Format(time.RFC3339Nano)),
	}
	p.mu.Lock()
	p.runtimeResources[kind] = item
	p.runtimeKind = kind
	p.runtimeSummary = item.Summary
	p.runtimeActive = status == "pending" || status == "running"
	p.mu.Unlock()
}

func (p *ProcessController) ForceKill() error {
	p.mu.RLock()
	command := p.cmd
	p.mu.RUnlock()
	if command == nil || command.Process == nil {
		return nil
	}
	treeErr := terminateProcessTree(command.Process.Pid)
	deadline := time.Now().Add(processKillWait)
	for p.IsRunning() && time.Now().Before(deadline) {
		time.Sleep(processExitPoll)
	}
	if p.IsRunning() {
		killErr := command.Process.Kill()
		deadline := time.Now().Add(processKillWait)
		for p.IsRunning() && time.Now().Before(deadline) {
			time.Sleep(processExitPoll)
		}
		if p.IsRunning() {
			return errors.Join(&BoundaryError{Code: "launcher.process_stop_failed", Message: "无法确认服务进程已停止。"}, treeErr, killErr)
		}
	}
	return nil
}

func (p *ProcessController) RunOffline(settings LauncherResolvedSettings, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), offlineOperationTimeout)
	defer cancel()
	commandArgs := append([]string{"-config", settings.ConfigPath}, args...)
	command := exec.CommandContext(ctx, settings.ServerExecutablePath, commandArgs...)
	command.Dir = settings.Workdir
	configureChildProcess(command)
	output, err := command.CombinedOutput()
	if err == nil {
		return nil
	}
	detail := sanitizeDiagnostic(string(output))
	if detail == "" {
		detail = err.Error()
	}
	return fmt.Errorf("离线命令执行失败：%s", detail)
}

func (p *ProcessController) IsRunning() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.cmd != nil
}

func (p *ProcessController) ProcessID() *int64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.cmd == nil || p.cmd.Process == nil {
		return nil
	}
	value := int64(p.cmd.Process.Pid)
	return &value
}

func (p *ProcessController) SetupToken() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.setupToken
}

func (p *ProcessController) ControlToken() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.controlToken
}

func (p *ProcessController) clearCredentials() {
	p.mu.Lock()
	p.setupToken = ""
	p.controlToken = ""
	p.mu.Unlock()
}

func (p *ProcessController) LogDirectory() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.logDirectory
}

func (p *ProcessController) SetWorkdir(workdir string) {
	if strings.TrimSpace(workdir) == "" {
		return
	}
	p.mu.Lock()
	p.logDirectory = filepath.Join(workdir, "logs")
	p.mu.Unlock()
}

func (p *ProcessController) RecentStderr() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return append([]string{}, p.stderr...)
}

func (p *ProcessController) RuntimePrepare() *RuntimePrepareSnapshot {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if len(p.runtimeResources) == 0 && !p.runtimeActive {
		return nil
	}
	resources := make([]RuntimePrepareResourceProgress, 0, len(p.runtimeResources))
	for _, item := range p.runtimeResources {
		resources = append(resources, item)
	}
	sort.Slice(resources, func(left, right int) bool {
		if resources[left].Kind == resources[right].Kind {
			return resources[left].ResourceID < resources[right].ResourceID
		}
		return resources[left].Kind < resources[right].Kind
	})
	return &RuntimePrepareSnapshot{Active: p.runtimeActive, CurrentKind: p.runtimeKind, Summary: p.runtimeSummary, Resources: resources}
}

func (p *ProcessController) ClearRuntimePrepare() {
	p.mu.Lock()
	p.runtimeResources = map[string]RuntimePrepareResourceProgress{}
	p.runtimeActive = false
	p.runtimeKind = ""
	p.runtimeSummary = ""
	p.mu.Unlock()
}

func (p *ProcessController) WriteLauncherLog(message, workdir string) {
	if strings.TrimSpace(workdir) != "" {
		p.mu.Lock()
		p.logDirectory = filepath.Join(workdir, "logs")
		p.mu.Unlock()
	}
	p.appendLog("launcher", "launcher", message+"\n")
}

func (p *ProcessController) recordDiagnostic(value string) {
	value = sanitizeDiagnostic(value)
	if value == "" {
		return
	}
	p.mu.Lock()
	p.stderr = append(p.stderr, value)
	if len(p.stderr) > maxRecentDiagnostics {
		overflow := len(p.stderr) - maxRecentDiagnostics
		copy(p.stderr, p.stderr[overflow:])
		p.stderr = p.stderr[:maxRecentDiagnostics]
	}
	p.mu.Unlock()
}

func (p *ProcessController) appendLog(scope, stream, text string) {
	if text == "" {
		return
	}
	p.logMu.Lock()
	defer p.logMu.Unlock()
	directory := filepath.Join(p.LogDirectory(), scope)
	if os.MkdirAll(directory, 0o755) != nil {
		return
	}
	now := time.Now()
	filePath := filepath.Join(directory, now.Format("2006-01-02")+".log")
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer file.Close()
	_, _ = fmt.Fprintf(file, "[%s] [%s] %s", now.Format(time.RFC3339), stream, text)
}

func secureToken() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func sanitizeDiagnostic(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > 500 {
		value = value[:497] + "..."
	}
	return value
}

func stringField(payload map[string]any, key string) string {
	value, _ := payload[key].(string)
	return strings.TrimSpace(value)
}

func floatPointer(value any) *float64 {
	number, ok := value.(float64)
	if !ok {
		return nil
	}
	return &number
}

func intPointer(value any) *int64 {
	number, ok := value.(float64)
	if !ok {
		return nil
	}
	converted := int64(number)
	return &converted
}

func currentWorkingDirectory() string {
	workingDirectory, err := os.Getwd()
	if err != nil {
		return "."
	}
	return workingDirectory
}
