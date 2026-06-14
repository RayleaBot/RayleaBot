package app

import apphost "github.com/RayleaBot/RayleaBot/server/internal/apphost"

type Options = apphost.Options
type App = apphost.App

func New(options Options) (*App, error) {
	return apphost.New(options)
}
