package bootstrap

import "github.com/RayleaBot/RayleaBot/server/internal/app"

type Options = app.Options

func New(options Options) (Application, error) {
	return app.New(options)
}
