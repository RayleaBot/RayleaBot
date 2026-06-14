package storageaction

import (
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginfile"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginkv"
)

const (
	defaultKVValueMaxBytes      = 65536
	defaultKVTotalLimitMegabyte = 16
	defaultFileMaxBytes         = 10 * 1024 * 1024
	defaultPluginWorkdirMB      = 256
)

func currentKVLimits(cfg config.Config) pluginkv.Limits {
	valueLimit := cfg.Storage.KVValueMaxBytes
	if valueLimit <= 0 {
		valueLimit = defaultKVValueMaxBytes
	}
	totalLimitMB := cfg.Storage.KVTotalLimitMB
	if totalLimitMB <= 0 {
		totalLimitMB = defaultKVTotalLimitMegabyte
	}
	return pluginkv.Limits{
		ValueMaxBytes: valueLimit,
		TotalMaxBytes: totalLimitMB * 1024 * 1024,
	}
}

func currentFileLimits(cfg config.Config) pluginfile.Limits {
	fileLimit := cfg.Storage.FileMaxBytes
	if fileLimit <= 0 {
		fileLimit = defaultFileMaxBytes
	}
	totalLimitMB := cfg.Storage.PluginWorkDirMB
	if totalLimitMB <= 0 {
		totalLimitMB = defaultPluginWorkdirMB
	}
	return pluginfile.Limits{
		FileMaxBytes:  fileLimit,
		TotalMaxBytes: totalLimitMB * 1024 * 1024,
	}
}
