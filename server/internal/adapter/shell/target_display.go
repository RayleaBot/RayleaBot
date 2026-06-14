package shell

import (
	"context"

	adapterapi "github.com/RayleaBot/RayleaBot/server/internal/adapter/api"
)

func (s *Shell) ResolveTargetName(ctx context.Context, targetType, targetID string) string {
	return adapterapi.ResolveTargetName(ctx, targetType, targetID, shellTargetNameResolver{s: s})
}

type shellTargetNameResolver struct {
	s *Shell
}

func (r shellTargetNameResolver) ResolveGroupName(ctx context.Context, targetID string) string {
	return r.s.resolveGroupName(ctx, targetID)
}

func (r shellTargetNameResolver) ResolvePrivateName(ctx context.Context, targetID string) string {
	return r.s.resolveStrangerInfo(ctx, targetID).Nickname
}
