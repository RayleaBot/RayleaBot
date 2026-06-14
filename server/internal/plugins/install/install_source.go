package install

import (
	"context"
	"errors"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"net/url"
	"os"
	"path/filepath"
)

func (s *InstallService) prepareSource(ctx context.Context, request plugins.InstallRequest) (string, string, func(), error) {
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
