package plugins

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"rayleabot/server/internal/tasks"
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

	candidateDir = ""

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
		s.afterSuccess(candidateSnapshot.PluginID)
	}

	_ = workingRoot
	return nil
}

func (s *InstallService) buildPackageMetadata(request InstallRequest, snapshot Snapshot, candidateDir string) (PackageMetadata, error) {
	manifestHash, err := s.deps.hashFile(filepath.Join(candidateDir, "info.json"))
	if err != nil {
		return PackageMetadata{}, installError(codePluginInstallFailed, "计算插件 manifest 哈希失败", "计算插件 manifest 哈希失败")
	}
	packageHash, err := s.deps.hashDir(candidateDir)
	if err != nil {
		return PackageMetadata{}, installError(codePluginInstallFailed, "计算插件安装包哈希失败", "计算插件安装包哈希失败")
	}

	return PackageMetadata{
		PluginID:     snapshot.PluginID,
		SourceType:   request.SourceType,
		SourceRef:    request.Source,
		Version:      snapshot.Version,
		ManifestHash: manifestHash,
		PackageHash:  packageHash,
	}, nil
}

func (s *InstallService) prepareDependencies(ctx context.Context, candidateDir string, snapshot Snapshot, allowInstallScripts bool) error {
	if snapshot.RequireInstallScripts && !allowInstallScripts {
		return installError("platform.install_script_blocked", "插件安装脚本被默认安全策略阻止", "插件安装脚本被默认安全策略阻止")
	}

	switch snapshot.Runtime {
	case "python":
		if err := s.deps.preparePython(ctx, candidateDir, snapshot.PythonDependencies); err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return err
			}
			return installError(codePluginInstallFailed, "准备 Python 插件依赖环境失败", "准备 Python 插件依赖环境失败")
		}
	case "nodejs":
		needsNodeSetup := len(snapshot.NodeDependencies) > 0 || snapshot.RequireInstallScripts
		if !needsNodeSetup {
			return nil
		}
		if snapshot.RequireInstallScripts {
			packageJSONPath := filepath.Join(candidateDir, "package.json")
			if _, err := s.deps.stat(packageJSONPath); err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return installError(codePluginInstallFailed, "插件声明需要安装脚本但 package.json 缺失", "插件声明需要安装脚本但 package.json 缺失")
				}
				return installError(codePluginInstallFailed, "检查 Node.js 插件 package.json 失败", "检查 Node.js 插件 package.json 失败")
			}
		}
		allowNodeScripts := allowInstallScripts && snapshot.RequireInstallScripts
		if err := s.deps.prepareNode(ctx, candidateDir, snapshot.NodeDependencies, allowNodeScripts); err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return err
			}
			return installError(codePluginInstallFailed, "准备 Node.js 插件依赖环境失败", "准备 Node.js 插件依赖环境失败")
		}
	}

	return nil
}

type installTaskError struct {
	Code    string
	Message string
	Summary string
}

func (e *installTaskError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func installError(code, message, summary string) error {
	return &installTaskError{
		Code:    code,
		Message: message,
		Summary: summary,
	}
}
