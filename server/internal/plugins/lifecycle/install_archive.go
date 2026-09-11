package lifecycle

import (
	"archive/zip"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/fsguard"
)

func extractZipSource(ctx context.Context, archivePath, tempRoot string) (string, error) {
	archiveInfo, err := os.Stat(archivePath)
	if err != nil {
		return "", installError(codePluginInstallFailed, "读取插件压缩包失败", "读取插件压缩包失败")
	}
	if archiveInfo.Size() > maxRemoteDownloadBytes {
		return "", installError(codePackageResourceLimit, "插件包超过资源限制", "插件包超过资源限制")
	}

	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", installError(codePluginInstallFailed, "解压插件压缩包失败", "解压插件压缩包失败")
	}
	defer func(release func() error) { _ = release() }(reader.Close)
	if len(reader.File) > maxPluginArchiveEntries {
		return "", installError(codePackageResourceLimit, "插件包超过资源限制", "插件包超过资源限制")
	}

	extractRoot := filepath.Join(tempRoot, "unzipped")
	if err := os.MkdirAll(extractRoot, 0o755); err != nil {
		return "", installError(codePluginInstallFailed, "创建解压临时目录失败", "创建解压临时目录失败")
	}

	topLevels := map[string]struct{}{}
	seenPaths := make(map[string]struct{}, len(reader.File))
	var expandedBytes uint64

	for _, file := range reader.File {
		if err := ctx.Err(); err != nil {
			return "", err
		}

		cleanName, err := validatePluginArchiveEntry(file, seenPaths, &expandedBytes)
		if err != nil {
			return "", err
		}

		targetPath := filepath.Join(extractRoot, cleanName)
		if !fsguard.WithinRoot(extractRoot, targetPath) {
			return "", installError(codePluginInstallFailed, "插件压缩包包含越界路径", "插件压缩包包含越界路径")
		}

		parts := strings.Split(filepath.ToSlash(cleanName), "/")
		if len(parts) > 0 && parts[0] != "." && parts[0] != "" {
			topLevels[parts[0]] = struct{}{}
		}

		if err := writePluginArchiveEntry(ctx, file, targetPath); err != nil {
			return "", err
		}
	}

	if len(topLevels) != 1 {
		return "", installError(codePluginInstallFailed, "压缩包必须只包含一个插件根目录", "压缩包必须只包含一个插件根目录")
	}

	var rootName string
	for name := range topLevels {
		rootName = name
	}

	rootPath := filepath.Join(extractRoot, filepath.FromSlash(rootName))
	info, err := os.Stat(rootPath)
	if err != nil || !info.IsDir() {
		return "", installError(codePluginInstallFailed, "压缩包必须只包含一个插件根目录", "压缩包必须只包含一个插件根目录")
	}
	return rootPath, nil
}

func validatePluginArchiveEntry(file *zip.File, seenPaths map[string]struct{}, expandedBytes *uint64) (string, error) {
	cleanName, pathErr := fsguard.ArchivePath(file.Name, true)
	if pathErr != nil {
		return "", installError(codePackageUnsafeEntry, "插件包包含不安全文件", "插件包包含不安全文件")
	}
	canonicalName := strings.ToLower(filepath.ToSlash(cleanName))
	if _, exists := seenPaths[canonicalName]; exists {
		return "", installError(codePackageUnsafeEntry, "插件包包含不安全文件", "插件包包含不安全文件")
	}
	seenPaths[canonicalName] = struct{}{}

	mode := file.Mode()
	if mode&(os.ModeSymlink|os.ModeDevice|os.ModeCharDevice|os.ModeNamedPipe|os.ModeSocket|os.ModeIrregular) != 0 {
		return "", installError(codePackageUnsafeEntry, "插件包包含不安全文件", "插件包包含不安全文件")
	}
	if !file.FileInfo().IsDir() {
		if file.UncompressedSize64 > maxPluginArchiveFileBytes {
			return "", installError(codePackageResourceLimit, "插件包超过资源限制", "插件包超过资源限制")
		}
		if exceedsCompressionRatio(file.UncompressedSize64, file.CompressedSize64, maxPluginArchiveRatio) {
			return "", installError(codePackageResourceLimit, "插件包超过资源限制", "插件包超过资源限制")
		}
		if *expandedBytes > maxPluginArchiveExpandBytes-file.UncompressedSize64 {
			return "", installError(codePackageResourceLimit, "插件包超过资源限制", "插件包超过资源限制")
		}
		*expandedBytes += file.UncompressedSize64
	}

	return cleanName, nil
}

func writePluginArchiveEntry(ctx context.Context, file *zip.File, targetPath string) error {
	if file.FileInfo().IsDir() {
		if err := os.MkdirAll(targetPath, normalizedZipEntryMode(file)); err != nil {
			return installError(codePluginInstallFailed, "创建解压目录失败", "创建解压目录失败")
		}
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return installError(codePluginInstallFailed, "创建解压目录失败", "创建解压目录失败")
	}

	readerHandle, err := file.Open()
	if err != nil {
		return installError(codePluginInstallFailed, "读取压缩包条目失败", "读取压缩包条目失败")
	}

	targetFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, normalizedZipEntryMode(file))
	if err != nil {
		_ = readerHandle.Close()
		return installError(codePluginInstallFailed, "写入解压文件失败", "写入解压文件失败")
	}

	copyErr := fsguard.CopyExact(ctx, targetFile, readerHandle, int64(file.UncompressedSize64))
	if err := errors.Join(copyErr, targetFile.Close(), readerHandle.Close()); err != nil {
		return installError(codePluginInstallFailed, "写入解压文件失败", "写入解压文件失败")
	}
	return nil
}

func normalizedZipEntryMode(file *zip.File) os.FileMode {
	mode := file.Mode().Perm()
	if file.FileInfo().IsDir() {
		if mode&0o111 == 0 {
			return 0o755
		}
		return mode
	}
	if mode == 0 {
		return 0o644
	}
	return mode
}

func exceedsCompressionRatio(uncompressed, compressed uint64, maxRatio uint64) bool {
	if uncompressed == 0 {
		return false
	}
	if compressed == 0 || maxRatio == 0 {
		return true
	}
	quotient := uncompressed / compressed
	return quotient > maxRatio || quotient == maxRatio && uncompressed%compressed != 0
}
