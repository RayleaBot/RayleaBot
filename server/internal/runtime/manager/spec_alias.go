package manager

import (
	"context"
	"errors"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	runtimespec "github.com/RayleaBot/RayleaBot/server/internal/runtime/spec"
)

type BotInfo = runtimespec.BotInfo
type InitPayload = runtimespec.InitPayload
type Spec = runtimespec.Spec

func BuildSpec(snapshot plugins.Snapshot, repoRoot string, runtimeConfig config.RuntimeConfig) (Spec, error) {
	return BuildSpecWithContext(context.Background(), snapshot, repoRoot, runtimeConfig)
}

func BuildSpecWithContext(ctx context.Context, snapshot plugins.Snapshot, repoRoot string, runtimeConfig config.RuntimeConfig) (Spec, error) {
	spec, err := runtimespec.BuildSpecWithContext(ctx, snapshot, repoRoot, runtimeConfig)
	if err != nil {
		return Spec{}, normalizeSpecError(err)
	}
	return spec, nil
}

func normalizeSpecError(err error) error {
	var specErr *runtimespec.Error
	if errors.As(err, &specErr) {
		return errorf(specErr.Code, specErr.Message, specErr.Err)
	}
	return err
}
