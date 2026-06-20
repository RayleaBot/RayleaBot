package deps

import "strings"

func ManagedResourceLabel(kind string) string {
	return managedResourceLabel(kind)
}

func ManagedResourceText(kind, suffix string) string {
	return managedResourceText(kind, suffix)
}

func BootstrapRemediation(kind, archivePath, storeRoot string) string {
	return bootstrapRemediation(kind, archivePath, storeRoot)
}

func BootstrapSummary(kind string, inspection *BootstrapInspection) string {
	switch {
	case inspection == nil:
		return managedResourceText(kind, "清单不可用。")
	case !inspection.MetadataComplete:
		return managedResourceText(kind, "元数据不完整。")
	case inspection.PreparedStorePresent:
		return managedResourceText(kind, "已准备完成。")
	case inspection.CachedArchivePresent:
		if kind == "python-runtime" || kind == "nodejs-runtime" {
			return managedResourceText(kind, "已下载，启动时会解压。")
		}
		return managedResourceText(kind, "已下载，未解压。")
	default:
		if kind == "python-runtime" || kind == "nodejs-runtime" {
			return managedResourceText(kind, "已纳入启动流程。")
		}
		return managedResourceText(kind, "未准备。")
	}
}

func bootstrapMessage(kind, stage string) string {
	switch stage {
	case "manifest":
		return managedResourceText(kind, "清单不可用")
	case "lock":
		return managedResourceText(kind, "准备锁等待超时")
	case "download":
		return managedResourceText(kind, "安装包下载失败")
	case "verify":
		return managedResourceText(kind, "安装包校验失败")
	case "extract":
		return managedResourceText(kind, "安装包解压失败")
	case "entrypoint":
		return managedResourceText(kind, "入口文件缺失")
	default:
		return managedResourceText(kind, "准备失败")
	}
}

func bootstrapRemediation(kind, archivePath, storeRoot string) string {
	paths := []string{}
	if strings.TrimSpace(archivePath) != "" {
		paths = append(paths, "下载位置："+archivePath+"。")
	}
	if strings.TrimSpace(storeRoot) != "" {
		paths = append(paths, "解压位置："+storeRoot+"。")
	}
	locationText := strings.Join(paths, "")
	switch kind {
	case "chromium":
		return "启动运行环境任务准备图片渲染 Chromium，或在配置中设置 render.browser_path。" + locationText
	case "python-runtime":
		return "启动运行环境任务准备 Python 运行环境。" + locationText
	case "nodejs-runtime":
		return "启动运行环境任务准备 Node.js 和 npm 环境。" + locationText
	default:
		return "启动运行环境任务准备依赖。" + locationText
	}
}

func managedResourceLabel(kind string) string {
	switch kind {
	case "chromium":
		return "图片渲染 Chromium"
	case "python-runtime":
		return "Python 运行环境"
	case "nodejs-runtime":
		return "Node.js / npm 环境"
	default:
		return "运行环境"
	}
}

func managedResourceText(kind, suffix string) string {
	label := managedResourceLabel(kind)
	if kind == "chromium" && strings.TrimSpace(suffix) != "" {
		return label + " " + suffix
	}
	return label + suffix
}
