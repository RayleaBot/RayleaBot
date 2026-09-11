package lifecycle

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/fsguard"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func downloadHTTPSFile(ctx context.Context, rawURL, destPath string) (err error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return fmt.Errorf("invalid HTTPS URL: %s", rawURL)
	}

	client := &http.Client{
		Timeout: 10 * time.Minute,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
		},
		CheckRedirect: validatePluginDownloadRedirect,
	}
	defer client.CloseIdleConnections()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func(release func() error) { _ = release() }(resp.Body.Close)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("remote server returned HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength > maxRemoteDownloadBytes {
		return fmt.Errorf("%w: download exceeded maximum size of %d bytes", errPluginPackageResourceLimit, maxRemoteDownloadBytes)
	}

	outFile, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, outFile.Close()) }()

	limitedReader := io.LimitReader(resp.Body, maxRemoteDownloadBytes+1)
	written, err := io.Copy(outFile, limitedReader)
	if err != nil {
		return err
	}
	if written > maxRemoteDownloadBytes {
		return fmt.Errorf("%w: download exceeded maximum size of %d bytes", errPluginPackageResourceLimit, maxRemoteDownloadBytes)
	}
	return nil
}

func validatePluginDownloadRedirect(request *http.Request, via []*http.Request) error {
	if len(via) > maxPluginDownloadRedirects {
		return errors.New("remote plugin download exceeded redirect limit")
	}
	if request == nil || request.URL == nil || request.URL.Scheme != "https" || request.URL.Host == "" || request.URL.User != nil {
		return errors.New("remote plugin redirect must use HTTPS without userinfo")
	}
	return nil
}

func (s *InstallService) prepareSource(ctx context.Context, request plugins.InstallRequest) (string, string, func(), error) {
	if err := os.MkdirAll(s.installedRoot, 0o755); err != nil {
		return "", "", func() {}, installError(codePluginInstallFailed, "创建插件安装目录失败", "创建插件安装目录失败")
	}
	tempRoot, err := s.deps.mkdirTemp(s.installedRoot, ".plugin-install-*")
	if err != nil {
		return "", "", func() {}, installError(codePluginInstallFailed, "创建安装临时目录失败", "创建安装临时目录失败")
	}
	cleanup := func() { _ = s.deps.removeAll(tempRoot) }
	candidate, err := s.materializeSource(ctx, request, tempRoot)
	if err != nil {
		cleanup()
		return "", "", func() {}, err
	}
	return tempRoot, candidate, cleanup, nil
}

func (s *InstallService) materializeSource(ctx context.Context, request plugins.InstallRequest, tempRoot string) (string, error) {
	sourceType, source := request.SourceType, request.Source
	if request.ResolvedSourceType != "" {
		sourceType, source = request.ResolvedSourceType, request.ResolvedSource
	}
	switch sourceType {
	case "local_directory":
		return s.prepareDirectorySource(ctx, source, tempRoot)
	case "local_zip":
		return s.prepareLocalArchiveSource(ctx, source, tempRoot, request.ExpectedArchiveSHA256)
	case "remote_url":
		return s.prepareRemoteArchiveSource(ctx, source, tempRoot, request.ExpectedArchiveSHA256)
	default:
		return "", installError(codeInvalidRequest, "插件来源类型不受支持", "插件来源类型不受支持")
	}
}

func (s *InstallService) prepareDirectorySource(ctx context.Context, source, tempRoot string) (string, error) {
	info, err := s.deps.stat(source)
	if errors.Is(err, os.ErrNotExist) {
		return "", installError(codeResourceMissing, "插件来源目录不存在", "插件来源目录不存在")
	}
	if err != nil {
		return "", installError(codePluginInstallFailed, "检查插件来源目录失败", "检查插件来源目录失败")
	}
	if !info.IsDir() {
		return "", installError(codeInvalidRequest, "插件来源必须是目录", "插件来源必须是目录")
	}
	candidate := filepath.Join(tempRoot, "candidate")
	if err := s.deps.copyDir(ctx, source, candidate); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return "", err
		}
		return "", installError(codePluginInstallFailed, "复制插件来源目录失败", "复制插件来源目录失败")
	}
	return candidate, nil
}

func (s *InstallService) prepareLocalArchiveSource(ctx context.Context, source, tempRoot, expectedDigest string) (string, error) {
	info, err := s.deps.stat(source)
	if errors.Is(err, os.ErrNotExist) {
		return "", installError(codeResourceMissing, "插件来源压缩包不存在", "插件来源压缩包不存在")
	}
	if err != nil {
		return "", installError(codePluginInstallFailed, "检查插件来源压缩包失败", "检查插件来源压缩包失败")
	}
	if info.IsDir() {
		return "", installError(codeInvalidRequest, "插件来源必须是压缩包文件", "插件来源必须是压缩包文件")
	}
	if err := verifyPluginSourceDigest(ctx, source, expectedDigest, "插件压缩包摘要与商店目录不一致"); err != nil {
		return "", err
	}
	return s.deps.extractZip(ctx, source, tempRoot)
}

func (s *InstallService) prepareRemoteArchiveSource(ctx context.Context, source, tempRoot, expectedDigest string) (string, error) {
	parsed, err := url.Parse(source)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return "", installError(codeInvalidRequest, "远程来源必须是 HTTPS URL", "远程来源必须是 HTTPS URL")
	}
	downloaded := filepath.Join(tempRoot, "download.zip")
	if err := s.deps.downloadFile(ctx, source, downloaded); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return "", err
		}
		if errors.Is(err, errPluginPackageResourceLimit) {
			return "", installError(codePackageResourceLimit, "插件包超过资源限制", "插件包超过资源限制")
		}
		return "", installError(codePluginInstallFailed, "下载远程插件压缩包失败", "下载远程插件压缩包失败")
	}
	if err := verifyPluginSourceDigest(ctx, downloaded, expectedDigest, "下载的插件摘要与商店目录不一致"); err != nil {
		return "", err
	}
	return s.deps.extractZip(ctx, downloaded, tempRoot)
}

func verifyPluginSourceDigest(ctx context.Context, source, expected, message string) error {
	if expected == "" {
		return nil
	}
	digest, err := fsguard.SHA256File(ctx, source, maxRemoteDownloadBytes)
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if err != nil || digest != expected {
		return installError(errorcodes.PluginStoreIntegrityMismatch, message, "插件商店产物完整性校验失败")
	}
	return nil
}
