package bootstrap

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/app"
)

type Options = app.Options

func New(options Options) (Application, error) {
	return app.New(options)
}

func NewWithContext(ctx context.Context, options Options) (Application, error) {
	return app.NewWithContext(ctx, options)
}
