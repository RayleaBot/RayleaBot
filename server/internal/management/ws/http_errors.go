package ws

import (
	"net/http"

	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
)

func writeWebSocketPermissionDenied(w http.ResponseWriter, r *http.Request) {
	httpapi.WriteError(
		w,
		r,
		http.StatusUnauthorized,
		"permission.denied",
		"当前用户无权执行该操作",
		"errors.permission.denied",
		nil,
	)
}

func writeWebSocketNotFound(w http.ResponseWriter, r *http.Request) {
	httpapi.WriteError(
		w,
		r,
		http.StatusNotFound,
		"platform.resource_missing",
		"缺少必要资源",
		"errors.platform.resource_missing",
		nil,
	)
}
