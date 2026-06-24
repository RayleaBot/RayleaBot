package bootstrap

import "net/http"

type Application interface {
	Runnable
	Handler() http.Handler
}
