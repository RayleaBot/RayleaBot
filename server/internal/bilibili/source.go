package bilibili

import bilibiliSource "github.com/RayleaBot/RayleaBot/server/internal/bilibili/source"

type Source = bilibiliSource.Source
type Deps = bilibiliSource.Deps
type Dispatcher = bilibiliSource.Dispatcher
type ProxyConfig = bilibiliSource.ProxyConfig
type ProxyPool = bilibiliSource.ProxyPool

func NewSource(deps Deps) (*Source, error) {
	return bilibiliSource.NewSource(deps)
}

func NewProxyPool(configs []ProxyConfig) *ProxyPool {
	return bilibiliSource.NewProxyPool(configs)
}
