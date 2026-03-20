package app

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/go-chi/chi/v5"

	"rayleabot/server/internal/console"
)

type consoleFrame struct {
	Channel   string           `json:"channel"`
	Type      string           `json:"type"`
	Timestamp string           `json:"timestamp"`
	Data      consoleFrameData `json:"data"`
}

type consoleFrameData struct {
	PluginID  string `json:"plugin_id"`
	Stream    string `json:"stream"`
	Text      string `json:"text"`
	Timestamp string `json:"timestamp"`
}

func (a *App) handlePluginConsoleWebSocket() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := ClaimsFromContext(r.Context()); !ok {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}

		pluginID := strings.TrimSpace(chi.URLParam(r, "id"))
		if pluginID == "" {
			http.NotFound(w, r)
			return
		}
		if _, ok := a.Plugins.Get(pluginID); !ok {
			http.NotFound(w, r)
			return
		}

		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer func() {
			_ = conn.Close(websocket.StatusNormalClosure, "")
		}()

		framesCtx := conn.CloseRead(context.Background())
		entries, unsubscribe := a.Console.Subscribe(pluginID, 8)
		defer unsubscribe()

		for _, entry := range a.Console.Snapshot(pluginID) {
			if err := wsjson.Write(framesCtx, conn, newConsoleFrame(entry)); err != nil {
				return
			}
		}

		for {
			select {
			case <-framesCtx.Done():
				return
			case entry, ok := <-entries:
				if !ok {
					return
				}
				if err := wsjson.Write(framesCtx, conn, newConsoleFrame(entry)); err != nil {
					return
				}
			}
		}
	}
}

func newConsoleFrame(entry console.Entry) consoleFrame {
	timestamp := entry.Timestamp.UTC().Format(time.RFC3339)
	return consoleFrame{
		Channel:   "plugin_console",
		Type:      "plugins.console",
		Timestamp: timestamp,
		Data: consoleFrameData{
			PluginID:  entry.PluginID,
			Stream:    entry.Stream,
			Text:      entry.Text,
			Timestamp: timestamp,
		},
	}
}
