package plugins

import (
	"context"
	"errors"
	"net/url"
	"os"
	"path/filepath"

	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

func (s *InstallService) prepareSource(ctx context.Context, request InstallRequest) (string, string, func(), error) {
	if err := os.MkdirAll(s.installedRoot, 0o755); err != nil {
		return "", "", func() {}, installError(codePluginInstallFailed, "创建插件安装目录失败", "创建插件安装目录失败")
	}
	tempRoot, err := s.deps.mkdirTemp(s.installedRoot, ".plugin-install-*")
	if err != nil {
		return "", "", func() {}, installError(codePluginInstallFailed, "创建安装临时目录失败", "创建安装临时目录失败")
	}

	cleanup := func() {
		_ = s.deps.removeAll(tempRoot)
	}

	switch request.SourceType {
	case "local_directory":
		info, err := s.deps.stat(request.Source)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				cleanup()
				return "", "", func() {}, installError(codeResourceMissing, "插件来源目录不存在", "插件来源目录不存在")
			}
			cleanup()
			return "", "", func() {}, installError(codePluginInstallFailed, "检查插件来源目录失败", "检查插件来源目录失败")
		}
		if !info.IsDir() {
			cleanup()
			return "", "", func() {}, installError(codeInvalidRequest, "插件来源必须是目录", "插件来源必须是目录")
		}

		candidate := filepath.Join(tempRoot, "candidate")
		if err := s.deps.copyDir(ctx, request.Source, candidate); err != nil {
			cleanup()
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return "", "", func() {}, err
			}
			return "", "", func() {}, installError(codePluginInstallFailed, "复制插件来源目录失败", "复制插件来源目录失败")
		}
		return tempRoot, candidate, cleanup, nil
	case "local_zip":
		info, err := s.deps.stat(request.Source)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				cleanup()
				return "", "", func() {}, installError(codeResourceMissing, "插件来源压缩包不存在", "插件来源压缩包不存在")
			}
			cleanup()
			return "", "", func() {}, installError(codePluginInstallFailed, "检查插件来源压缩包失败", "检查插件来源压缩包失败")
		}
		if info.IsDir() {
			cleanup()
			return "", "", func() {}, installError(codeInvalidRequest, "插件来源必须是压缩包文件", "插件来源必须是压缩包文件")
		}

		candidate, err := s.deps.extractZip(ctx, request.Source, tempRoot)
		if err != nil {
			cleanup()
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return "", "", func() {}, err
			}
			return "", "", func() {}, err
		}
		return tempRoot, candidate, cleanup, nil
	case "remote_url":
		parsed, err := url.Parse(request.Source)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
			cleanup()
			return "", "", func() {}, installError(codeInvalidRequest, "远程来源必须是 HTTPS URL", "远程来源必须是 HTTPS URL")
		}

		downloadPath := filepath.Join(tempRoot, "download.zip")
		if err := s.deps.downloadFile(ctx, request.Source, downloadPath); err != nil {
			cleanup()
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return "", "", func() {}, err
			}
			return "", "", func() {}, installError(codePluginInstallFailed, "下载远程插件压缩包失败", "下载远程插件压缩包失败")
		}

		candidate, err := s.deps.extractZip(ctx, downloadPath, tempRoot)
		if err != nil {
			cleanup()
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return "", "", func() {}, err
			}
			return "", "", func() {}, err
		}
		return tempRoot, candidate, cleanup, nil
	default:
		cleanup()
		return "", "", func() {}, installError(codeInvalidRequest, "插件来源类型不受支持", "插件来源类型不受支持")
	}
}

func (s *InstallService) loadCandidateSnapshot(candidateDir string) (Snapshot, error) {
	infoPath := filepath.Join(candidateDir, "info.json")
	snapshot, ok, err := loadSnapshot(infoPath, "plugins/installed", s.repoRoot, s.validator, validationMaxSummary, s.logger)
	if err != nil {
		return Snapshot{}, installError(codePluginInstallFailed, "读取插件 manifest 失败", "读取插件 manifest 失败")
	}
	if !ok {
		return Snapshot{}, installError(codeInvalidRequest, "插件 manifest 缺少必需字段", "插件 manifest 缺少必需字段")
	}
	if !snapshot.Valid {
		return Snapshot{}, installError(codePluginInstallFailed, snapshot.ValidationSummary, "插件 manifest 校验失败")
	}
	if snapshot.PluginID == "" {
		return Snapshot{}, installError(codeInvalidRequest, "插件 manifest 缺少插件 ID", "插件 manifest 缺少插件 ID")
	}
	return snapshot, nil
}

func (s *InstallService) refreshCatalog() error {
	snapshots, _, err := Discover(DiscoverOptions{
		Validator: s.validator,
		Roots:     s.discoveryRoots,
		RepoRoot:  s.repoRoot,
		Logger:    s.logger,
	})
	if err != nil {
		return installError(codePluginInstallFailed, "刷新插件目录索引失败", "刷新插件目录索引失败")
	}

	reloaded := NewCatalog(snapshots)
	if packageLoader, ok := s.repository.(PackageMetadataLoader); ok {
		packageMetadata, err := packageLoader.LoadAllPackageMetadata(context.Background())
		if err != nil {
			return installError(codePluginInstallFailed, "读取插件安装元数据失败", "读取插件安装元数据失败")
		}
		reloaded.Replace(ApplyPackageMetadata(reloaded.List(), packageMetadata))
	}
	if s.repository != nil {
		states, err := s.repository.LoadDesiredStates(context.Background())
		if err != nil {
			return installError(codePluginInstallFailed, "读取插件持久化状态失败", "读取插件持久化状态失败")
		}
		reloaded.ApplyDesiredStates(states)
	}

	s.catalog.Replace(reloaded.List())
	return nil
}

func (s *InstallService) failTask(taskID, code, message, summary string) {
	now := s.deps.now().UTC()
	s.registry.Update(taskID, tasks.Update{
		Status:     taskStatusPtr(tasks.StatusFailed),
		Summary:    stringPtr(summary),
		FinishedAt: &now,
		Error: &tasks.ErrorSummary{
			Code:    code,
			Message: message,
		},
	})
}

func (s *InstallService) dropCancel(taskID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.cancels, taskID)
}
