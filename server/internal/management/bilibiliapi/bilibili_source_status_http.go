package bilibiliapi

import (
	"net/http"

	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
)

func (h *BilibiliHandlers) HandleBilibiliSourceStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		httpapi.WriteJSON(w, http.StatusOK, bilibiliSourceStatusResponseFrom(h.source.Status(r.Context())))
	}
}

func (h *BilibiliHandlers) HandleBilibiliSourceRestart() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		httpapi.WriteJSON(w, http.StatusOK, bilibiliSourceRestartResponse{
			Accepted: true,
			Status:   bilibiliSourceStatusResponseFrom(h.source.Restart()),
		})
	}
}
