package app

import "net/http"

func (a *App) Handler() http.Handler {
	return a.process.router
}
