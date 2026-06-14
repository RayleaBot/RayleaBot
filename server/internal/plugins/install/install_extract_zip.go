package install

import (
	"archive/zip"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func extractZipSource(ctx context.Context, archivePath, tempRoot string) (string, error) {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", installError(codePluginInstallFailed, "解压插件压缩包失败", "解压插件压缩包失败")
	}
	defer reader.Close()

	extractRoot := filepath.Join(tempRoot, "unzipped")
	if err := os.MkdirAll(extractRoot, 0o755); err != nil {
		return "", installError(codePluginInstallFailed, "创建解压临时目录失败", "创建解压临时目录失败")
	}

	topLevels := map[string]struct{}{}

	for _, file := range reader.File {
		if err := ctx.Err(); err != nil {
			return "", err
		}

		cleanName := filepath.Clean(file.Name)
		if filepath.IsAbs(cleanName) || strings.HasPrefix(cleanName, "..") {
			return "", installError(codePluginInstallFailed, "插件压缩包包含越界路径", "插件压缩包包含越界路径")
		}

		targetPath := filepath.Join(extractRoot, cleanName)
		relativePath, err := filepath.Rel(extractRoot, targetPath)
		if err != nil || relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(filepath.Separator)) {
			return "", installError(codePluginInstallFailed, "插件压缩包包含越界路径", "插件压缩包包含越界路径")
		}

		parts := strings.Split(filepath.ToSlash(cleanName), "/")
		if len(parts) > 0 && parts[0] != "." && parts[0] != "" {
			topLevels[parts[0]] = struct{}{}
		}

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, normalizedZipEntryMode(file)); err != nil {
				return "", installError(codePluginInstallFailed, "创建解压目录失败", "创建解压目录失败")
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return "", installError(codePluginInstallFailed, "创建解压目录失败", "创建解压目录失败")
		}

		readerHandle, err := file.Open()
		if err != nil {
			return "", installError(codePluginInstallFailed, "读取压缩包条目失败", "读取压缩包条目失败")
		}

		targetFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, normalizedZipEntryMode(file))
		if err != nil {
			readerHandle.Close()
			return "", installError(codePluginInstallFailed, "写入解压文件失败", "写入解压文件失败")
		}

		if _, err := io.Copy(targetFile, readerHandle); err != nil {
			targetFile.Close()
			readerHandle.Close()
			return "", installError(codePluginInstallFailed, "写入解压文件失败", "写入解压文件失败")
		}

		targetFile.Close()
		readerHandle.Close()
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
