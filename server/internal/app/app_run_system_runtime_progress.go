package app

import (
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/deps"
)

func managedRuntimeTaskProgress(total, index int, event deps.PrepareProgress) (int, string) {
	if total <= 0 {
		total = 1
	}
	base := (index * 100) / total
	share := 100 / total
	stageProgress := event.Progress
	if stageProgress < 0 {
		stageProgress = 0
	}
	if stageProgress > 100 {
		stageProgress = 100
	}
	percent := base + (share*stageProgress)/100
	if percent > 99 && event.Status != "succeeded" {
		percent = 99
	}
	if percent > 100 {
		percent = 100
	}
	summary := strings.TrimSpace(event.Summary)
	if summary == "" {
		summary = runtimePrepareStageSummary(event)
	}
	return percent, summary
}

func runtimePrepareStageSummary(event deps.PrepareProgress) string {
	label := strings.TrimSpace(event.Label)
	if label == "" {
		label = deps.ManagedResourceLabel(event.Kind)
	}
	switch event.Stage {
	case "probe":
		return "正在测试 " + label + "下载来源"
	case "download":
		if event.Status == "succeeded" {
			return label + "安装包已下载"
		}
		return "正在下载 " + label
	case "verify":
		return "正在校验 " + label + "安装包"
	case "extract":
		if event.Status == "succeeded" {
			return label + "已解压"
		}
		return "正在解压 " + label
	case "cleanup":
		return "正在清理未完成的 " + label + "目录"
	case "activate":
		if event.Status == "succeeded" {
			return label + "已启用"
		}
		return "正在启用 " + label
	case "complete":
		return label + "已准备完成"
	default:
		return "正在准备 " + label
	}
}
