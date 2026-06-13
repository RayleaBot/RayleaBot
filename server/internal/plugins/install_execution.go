package plugins

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

func (s *InstallService) runInstall(job installJob) error {
	workingRoot, candidateDir, cleanup, err := s.prepareSource(job.ctx, job.request)
	if err != nil {
		return err
	}
	defer cleanup()

	if err := job.ctx.Err(); err != nil {
		return err
	}

	s.registry.Update(job.taskID, tasks.Update{
		Progress: intPtr(20),
		Summary:  stringPtr("校验插件 manifest"),
	})

	candidateSnapshot, err := s.loadCandidateSnapshot(candidateDir)
	if err != nil {
		return err
	}
	metadata, err := s.buildPackageMetadata(job.request, candidateSnapshot, candidateDir)
	if err != nil {
		return err
	}
	if _, exists := s.catalog.Get(candidateSnapshot.PluginID); exists {
		return installError(codePluginInstallFailed, "检测到同 ID 插件，安装被拒绝", "检测到同 ID 插件")
	}
	if s.validateRenderTemplates != nil {
		if err := s.validateRenderTemplates(candidateSnapshot); err != nil {
			return installError(codePluginInstallFailed, err.Error(), "插件渲染模板校验失败")
		}
	}

	if err := job.ctx.Err(); err != nil {
		return err
	}

	s.registry.Update(job.taskID, tasks.Update{
		Progress: intPtr(40),
		Summary:  stringPtr("准备插件依赖环境"),
	})

	if err := s.prepareDependencies(job.ctx, candidateDir, candidateSnapshot, job.request.AllowInstallScripts); err != nil {
		return err
	}

	s.registry.Update(job.taskID, tasks.Update{
		Progress: intPtr(60),
		Summary:  stringPtr("写入正式安装目录"),
	})

	if err := os.MkdirAll(s.installedRoot, 0o755); err != nil {
		return installError(codePluginInstallFailed, "创建插件安装目录失败", "创建插件安装目录失败")
	}

	finalTarget := filepath.Join(s.installedRoot, candidateSnapshot.PluginID)
	if _, err := s.deps.stat(finalTarget); err == nil {
		return installError(codePluginInstallFailed, "检测到同 ID 插件，安装被拒绝", "检测到同 ID 插件")
	} else if !errors.Is(err, os.ErrNotExist) {
		return installError(codePluginInstallFailed, "检查插件安装目录失败", "检查插件安装目录失败")
	}

	if err := s.deps.rename(candidateDir, finalTarget); err != nil {
		return installError(codePluginInstallFailed, "写入插件安装目录失败", "写入插件安装目录失败")
	}

	s.registry.Update(job.taskID, tasks.Update{
		Progress: intPtr(75),
		Summary:  stringPtr("刷新插件目录索引"),
	})

	if err := s.refreshCatalog(); err != nil {
		_ = s.deps.removeAll(finalTarget)
		return err
	}

	s.registry.Update(job.taskID, tasks.Update{
		Progress: intPtr(90),
		Summary:  stringPtr("写入安装元数据"),
	})

	if s.packageRepo != nil {
		metadata.InstalledAt = s.deps.now().UTC()
		if err := s.packageRepo.SavePackageMetadata(job.ctx, metadata); err != nil {
			_ = s.deps.removeAll(finalTarget)
			_ = s.refreshCatalog()
			return installError(codePluginInstallFailed, "写入插件安装元数据失败", "写入插件安装元数据失败")
		}
	}
	if s.afterSuccess != nil {
		if err := s.afterSuccess(candidateSnapshot.PluginID); err != nil {
			if s.packageRepo != nil {
				_ = s.packageRepo.DeletePackageMetadata(job.ctx, candidateSnapshot.PluginID)
			}
			_ = s.deps.removeAll(finalTarget)
			_ = s.refreshCatalog()
			return installError(codePluginInstallFailed, err.Error(), "插件安装后处理失败")
		}
	}

	_ = workingRoot
	return nil
}
