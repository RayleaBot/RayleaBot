package plugins

import "context"

type runtimeDoneKey struct{}
type parentRequestKey struct{}
type expectedRuntimeKey struct{}

// WithRuntimeDone carries process ownership alongside an event context. Host
// resources may outlive an event, but must not outlive that process generation.
func WithRuntimeDone(ctx context.Context, done <-chan struct{}) context.Context {
	return context.WithValue(ctx, runtimeDoneKey{}, done)
}

func RuntimeDone(ctx context.Context) <-chan struct{} {
	done, _ := ctx.Value(runtimeDoneKey{}).(<-chan struct{})
	return done
}

func WithParentRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, parentRequestKey{}, id)
}
func ParentRequestID(ctx context.Context) string {
	id, _ := ctx.Value(parentRequestKey{}).(string)
	return id
}
func WithExpectedRuntimeDone(ctx context.Context, done <-chan struct{}) context.Context {
	return context.WithValue(ctx, expectedRuntimeKey{}, done)
}
func ExpectedRuntimeDone(ctx context.Context) <-chan struct{} {
	done, _ := ctx.Value(expectedRuntimeKey{}).(<-chan struct{})
	return done
}
