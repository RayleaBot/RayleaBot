package pluginuninstall

import (
	"path/filepath"

	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

func (s *UninstallService) execute(job uninstallJob) {
	defer s.dropCancel(job.taskID)

	startedAt := s.deps.now().UTC()
	s.registry.Update(job.taskID, tasks.Update{
		Status:    taskStatusPtr(tasks.StatusRunning),
		Progress:  intPtr(10),
		Summary:   stringPtr("停止插件运行时"),
		StartedAt: &startedAt,
	})

	if s.stopPlugin != nil {
		s.stopPlugin(job.pluginID)
	}

	if err := job.ctx.Err(); err != nil {
		s.failTask(job.taskID, codePluginUninstallFailed, "插件卸载已取消", "插件卸载已取消")
		return
	}

	s.registry.Update(job.taskID, tasks.Update{
		Progress: intPtr(30),
		Summary:  stringPtr("清理数据库记录"),
	})

	if s.repository != nil {
		if err := s.repository.DeleteDesiredState(job.ctx, job.pluginID); err != nil {
			s.logger.Warn("delete desired_state during uninstall", "plugin_id", job.pluginID, "err", err.Error())
		}
	}

	if s.packageRepo != nil {
		if err := s.packageRepo.DeletePackageMetadata(job.ctx, job.pluginID); err != nil {
			s.logger.Warn("delete package metadata during uninstall", "plugin_id", job.pluginID, "err", err.Error())
		}
	}

	s.registry.Update(job.taskID, tasks.Update{
		Progress: intPtr(50),
		Summary:  stringPtr("删除插件安装目录"),
	})

	pluginDir := filepath.Join(s.installedRoot, job.pluginID)
	if _, err := s.deps.stat(pluginDir); err == nil {
		if err := s.deps.removeAll(pluginDir); err != nil {
			s.failTask(job.taskID, codePluginUninstallFailed, "删除插件安装目录失败", "删除插件安装目录失败")
			return
		}
	}

	s.registry.Update(job.taskID, tasks.Update{
		Progress: intPtr(80),
		Summary:  stringPtr("刷新插件目录索引"),
	})

	if err := s.refreshCatalog(); err != nil {
		s.failTask(job.taskID, codePluginUninstallFailed, "刷新插件目录索引失败", "刷新插件目录索引失败")
		return
	}
	if s.afterSuccess != nil {
		s.afterSuccess(job.pluginID)
	}

	now := s.deps.now().UTC()
	s.registry.Update(job.taskID, tasks.Update{
		Status:     taskStatusPtr(tasks.StatusSucceeded),
		Progress:   intPtr(100),
		Summary:    stringPtr("插件卸载完成"),
		FinishedAt: &now,
		Result: &tasks.ResultSummary{
			Summary: "插件已卸载并刷新插件目录索引",
		},
	})
}
