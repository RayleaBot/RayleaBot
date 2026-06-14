package adapter

import "context"

type TargetDisplayResolver interface {
	ResolveTargetName(context.Context, string, string) string
}
