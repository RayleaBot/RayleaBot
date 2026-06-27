package cli

import "github.com/RayleaBot/RayleaBot/server/internal/logpath"

func displayLogPath(repoRoot, path string) string {
	return logpath.Display(repoRoot, path)
}

func displayLogError(repoRoot string, err error, paths ...string) string {
	return logpath.Error(repoRoot, err, paths...)
}
