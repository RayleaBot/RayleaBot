package adapter

import "context"

func (s *Shell) ResolveTargetName(ctx context.Context, targetType, targetID string) string {
	switch targetType {
	case "group":
		return s.resolveGroupName(ctx, targetID)
	case "private":
		return s.resolveStrangerInfo(ctx, targetID).Nickname
	default:
		return ""
	}
}
