package plugins

import "context"

type runtimeDoneKey struct{}

// WithRuntimeDone carries process ownership alongside an event context. Host
// resources may outlive an event, but must not outlive that process generation.
func WithRuntimeDone(ctx context.Context, done <-chan struct{}) context.Context {
	return context.WithValue(ctx, runtimeDoneKey{}, done)
}

func RuntimeDone(ctx context.Context) <-chan struct{} {
	done, _ := ctx.Value(runtimeDoneKey{}).(<-chan struct{})
	return done
}
