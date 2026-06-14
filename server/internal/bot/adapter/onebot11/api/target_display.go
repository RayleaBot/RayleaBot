package api

import "context"

type TargetNameResolver interface {
	ResolveGroupName(context.Context, string) string
	ResolvePrivateName(context.Context, string) string
}

func ResolveTargetName(ctx context.Context, targetType, targetID string, resolver TargetNameResolver) string {
	if resolver == nil {
		return ""
	}
	switch targetType {
	case "group":
		return resolver.ResolveGroupName(ctx, targetID)
	case "private":
		return resolver.ResolvePrivateName(ctx, targetID)
	default:
		return ""
	}
}
