package bootstrap

import "github.com/RayleaBot/RayleaBot/server/internal/app"

func New(options app.Options) (Application, error) {
	return app.New(options)
}
