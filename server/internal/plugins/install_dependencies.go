package plugins

import (
	"context"
	"errors"
	"os"
	"path/filepath"
)

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
