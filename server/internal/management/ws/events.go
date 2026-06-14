package managementws

import (
	"net/http"

	"github.com/coder/websocket"

	authhttp "github.com/RayleaBot/RayleaBot/server/internal/management/authhttp"
)

func (h *EventsHandler) HandleEventsWebSocket() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authhttp.ClaimsFromContext(r.Context()); !ok {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}

		conn, err := acceptManagementWebSocket(w, r)
		if err != nil {
			return
		}
		defer func() {
			_ = conn.Close(websocket.StatusNormalClosure, "")
		}()

		h.streamEventsWebSocket(conn)
	}
}
