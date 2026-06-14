package managementws

import (
	"net/http"

	"github.com/coder/websocket"

	managementhttp "github.com/RayleaBot/RayleaBot/server/internal/management/http"
)

func (h *EventsHandler) HandleEventsWebSocket() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := managementhttp.ClaimsFromContext(r.Context()); !ok {
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
