package managementws

import (
	"net/http"

	"github.com/coder/websocket"
)

var managementWebSocketAcceptOptions = &websocket.AcceptOptions{
	OriginPatterns: []string{
		"127.0.0.1:4173",
		"localhost:4173",
	},
}

func acceptManagementWebSocket(w http.ResponseWriter, r *http.Request) (*websocket.Conn, error) {
	return websocket.Accept(w, r, managementWebSocketAcceptOptions)
}
