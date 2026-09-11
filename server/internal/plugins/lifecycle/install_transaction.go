package lifecycle

import (
	"context"
	"errors"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/artifact"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
	"os"
	"path/filepath"
)

type installTransaction struct {
	service                          *InstallService
	job                              installJob
	snapshot                         plugins.Snapshot
	metadata                         plugins.PackageMetadata
	finalTarget, previousTarget      string
	exists, replacing, previousMoved bool
	previousMetadata                 plugins.PackageMetadata
	hadPreviousMetadata              bool
}

func (s *InstallService) runInstall(job installJob) error {
	if job.inspection == nil {
		return installError(codeInvalidRequest, "插件安装缺少有效检查结果", "插件安装缺少有效检查结果")
	}
	if err := job.ctx.Err(); err != nil {
		return err
	}
	s.registry.Update(job.taskID, tasks.Update{Progress: intPtr(20), Summary: stringPtr("检查插件配置")})
	operationCtx, release, err := s.operations.Acquire(job.ctx, job.inspection.snapshot.PluginID)
	if err != nil {
		return err
	}
	defer release()
	job.ctx = operationCtx
	tx := installTransaction{service: s, job: job, snapshot: job.inspection.snapshot, metadata: job.inspection.metadata}
	if err := tx.validateCandidate(); err != nil {
		return err
	}
	if err := tx.prepareTarget(); err != nil {
		return err
	}
	if err := tx.activateFiles(); err != nil {
		return err
	}
	if stage, err := tx.finalize(); err != nil {
		return tx.rollback(stage, err)
	}
	return nil
}

func (tx *installTransaction) validateCandidate() error {
	s, job := tx.service, tx.job
	existing, exists := s.catalog.Get(tx.snapshot.PluginID)
	tx.exists = exists
	if exists && !job.request.ReplaceExisting {
		return installError(codePluginInstallFailed, "检测到同 ID 插件，安装被拒绝", "检测到同 ID 插件")
	}
	if exists && existing.SourceRoot != "plugins/installed" {
		return installError(codePluginInstallFailed, "同 ID 插件不属于统一安装目录", "同 ID 插件无法原子替换")
	}
	if s.validateRenderTemplates != nil {
		if err := s.validateRenderTemplates(tx.snapshot); err != nil {
			return installError(codePluginInstallFailed, err.Error(), "插件渲染模板校验失败")
		}
	}
	if err := job.ctx.Err(); err != nil {
		return err
	}
	s.registry.Update(job.taskID, tasks.Update{Progress: intPtr(40), Summary: stringPtr("检查插件安装包")})
	platform, err := artifact.CurrentPlatform()
	if err != nil {
		return installError(codePluginPlatformMismatch, err.Error(), "插件包与当前平台不匹配")
	}
	if _, err := artifact.Verify(job.inspection.candidateDir, artifact.Options{ExpectedPlatform: platform}); err != nil {
		if errors.Is(err, artifact.ErrPlatformMismatch) {
			return installError(codePluginPlatformMismatch, err.Error(), "插件包与当前平台不匹配")
		}
		return installError(codePluginArtifactInvalid, err.Error(), "插件 artifact 校验失败")
	}
	return nil
}

func (tx *installTransaction) prepareTarget() error {
	s, job := tx.service, tx.job
	s.registry.Update(job.taskID, tasks.Update{Progress: intPtr(60), Summary: stringPtr("写入正式安装目录")})
	if err := os.MkdirAll(s.installedRoot, 0o755); err != nil {
		return installError(codePluginInstallFailed, "创建插件安装目录失败", "创建插件安装目录失败")
	}
	tx.finalTarget = filepath.Join(s.installedRoot, tx.snapshot.PluginID)
	tx.previousTarget = filepath.Join(job.inspection.workingRoot, "previous")
	_, err := s.deps.stat(tx.finalTarget)
	if err == nil && !job.request.ReplaceExisting {
		return installError(codePluginInstallFailed, "检测到同 ID 插件，安装被拒绝", "检测到同 ID 插件")
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return installError(codePluginInstallFailed, "检查插件安装目录失败", "检查插件安装目录失败")
	}
	tx.replacing = err == nil
	if !tx.replacing || s.packageRepo == nil {
		return nil
	}
	loader, ok := s.repository.(plugins.PackageMetadataLoader)
	if !ok {
		return installError(codePluginInstallFailed, "安装元数据仓库不支持更新回滚", "插件更新未写入")
	}
	all, err := loader.LoadAllPackageMetadata(job.ctx)
	if err != nil {
		return installError(codePluginInstallFailed, "读取当前插件安装元数据失败", "插件更新未写入")
	}
	tx.previousMetadata, tx.hadPreviousMetadata = all[tx.snapshot.PluginID]
	return nil
}

func (tx *installTransaction) activateFiles() error {
	s, job := tx.service, tx.job
	if tx.replacing {
		if s.beforeReplace != nil {
			if err := s.beforeReplace(job.ctx, tx.snapshot.PluginID); err != nil {
				failure := &operationError{state: "unchanged"}
				failure.add("stop", err)
				return failure
			}
		}
		if err := s.renameInstallPath(job.ctx, tx.finalTarget, tx.previousTarget); err != nil {
			return tx.activationFailure("backup", err)
		}
		tx.previousMoved = true
	}
	if err := s.renameInstallPath(job.ctx, job.inspection.candidateDir, tx.finalTarget); err != nil {
		return tx.activationFailure("install", err)
	}
	return nil
}

func (tx *installTransaction) resumePrevious(ctx context.Context) error {
	if tx.service.afterRollback != nil {
		return tx.service.afterRollback(ctx, tx.snapshot.PluginID)
	}
	return nil
}

func (tx *installTransaction) activationFailure(stage string, cause error) error {
	failure := &operationError{state: "unchanged"}
	failure.add(stage, cause)
	if !tx.replacing {
		return failure
	}
	failure.state = "rolled_back"
	ctx, cancel := context.WithTimeout(context.WithoutCancel(tx.job.ctx), tx.service.timeout)
	defer cancel()
	if tx.previousMoved {
		if err := tx.service.renameInstallPath(ctx, tx.previousTarget, tx.finalTarget); err != nil {
			failure.add("rollback_files", err)
			failure.state = "rollback_failed"
			return failure
		}
	}
	failure.add("rollback_finalize", tx.resumePrevious(ctx))
	if len(failure.failures) > 1 {
		failure.state = "rollback_failed"
	}
	return failure
}

func (tx *installTransaction) finalize() (string, error) {
	s, job := tx.service, tx.job
	if s.packageRepo != nil {
		tx.metadata.InstalledAt = s.deps.now().UTC()
		if err := s.packageRepo.SavePackageMetadata(job.ctx, tx.metadata); err != nil {
			return "metadata", err
		}
	}
	s.registry.Update(job.taskID, tasks.Update{Progress: intPtr(75), Summary: stringPtr("刷新插件目录索引")})
	if !tx.exists && job.request.SourceType == "development" && s.repository != nil {
		if err := s.repository.SaveDesiredState(job.ctx, tx.snapshot.PluginID, plugins.DesiredStateEnabled, s.deps.now().UTC()); err != nil {
			return "desired_state", err
		}
	}
	if err := s.refreshCatalog(job.ctx, tx.snapshot.PluginID); err != nil {
		return "catalog", err
	}
	s.registry.Update(job.taskID, tasks.Update{Progress: intPtr(90), Summary: stringPtr("写入安装元数据")})
	if s.afterSuccess != nil {
		if err := s.afterSuccess(job.ctx, tx.snapshot.PluginID); err != nil {
			return "finalize", err
		}
	}
	return "", nil
}

func (tx *installTransaction) rollback(stage string, cause error) error {
	s, job := tx.service, tx.job
	failure := &operationError{state: "rolled_back"}
	failure.add(stage, cause)
	ctx, cancel := context.WithTimeout(context.WithoutCancel(job.ctx), s.timeout)
	defer cancel()
	// Initialization may have created a process; stop it before touching its files.
	if s.beforeReplace != nil {
		if err := s.beforeReplace(ctx, tx.snapshot.PluginID); err != nil {
			failure.state = "rollback_failed"
			failure.add("rollback_stop", err)
			return failure
		}
	}
	if !tx.exists && job.request.SourceType == "development" && s.repository != nil {
		failure.add("rollback_desired_state", s.repository.DeleteDesiredState(ctx, tx.snapshot.PluginID))
	}
	filesErr := tx.restoreFiles(ctx)
	failure.add("rollback_files", filesErr)
	failure.add("rollback_metadata", tx.restoreMetadata(ctx))
	catalogErr := s.refreshCatalog(ctx, tx.snapshot.PluginID)
	failure.add("rollback_catalog", catalogErr)
	// Do not restart a candidate or use a stale catalog after failed restoration.
	if filesErr == nil && catalogErr == nil {
		failure.add("rollback_finalize", tx.resumePrevious(ctx))
	}
	if len(failure.failures) > 1 {
		failure.state = "rollback_failed"
	}
	return failure
}

func (tx *installTransaction) restoreFiles(ctx context.Context) error {
	if err := tx.service.deps.removeAll(tx.finalTarget); err != nil {
		return err
	}
	if tx.replacing {
		return tx.service.renameInstallPath(ctx, tx.previousTarget, tx.finalTarget)
	}
	return nil
}

func (tx *installTransaction) restoreMetadata(ctx context.Context) error {
	if tx.service.packageRepo == nil {
		return nil
	}
	if tx.hadPreviousMetadata {
		return tx.service.packageRepo.SavePackageMetadata(ctx, tx.previousMetadata)
	}
	return tx.service.packageRepo.DeletePackageMetadata(ctx, tx.snapshot.PluginID)
}
