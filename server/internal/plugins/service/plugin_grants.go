package service

import plugingrants "github.com/RayleaBot/RayleaBot/server/internal/plugins/grants"

type GrantView = plugingrants.View

type GrantViewDeps = plugingrants.ViewDeps

func NewGrantView(deps GrantViewDeps) *GrantView {
	return plugingrants.NewView(deps)
}
