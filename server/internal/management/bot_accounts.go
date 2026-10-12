package management

import (
	"errors"
	"net/http"
	"net/url"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/adapters"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/httpapi"
	"github.com/go-chi/chi/v5"
)

type BotAccountHandlers struct{ directory *adapters.BotAccountDirectory }

func NewBotAccountHandlers(directory *adapters.BotAccountDirectory) *BotAccountHandlers {
	return &BotAccountHandlers{directory: directory}
}

func (h *BotAccountHandlers) RegisterProtectedRoutes(router chi.Router) {
	router.Get("/api/bot-accounts", h.list)
	router.Get("/api/bot-accounts/{source_protocol}/{bot_id}", h.get)
	router.Delete("/api/bot-accounts/{source_protocol}/{bot_id}", h.delete)
}

func (h *BotAccountHandlers) list(w http.ResponseWriter, r *http.Request) {
	query, ok := readCollectionQuery(w, r)
	if !ok {
		return
	}
	protocol, ok := readCollectionChoice(w, r, "source_protocol", "onebot11", "qqofficial")
	if !ok {
		return
	}
	state, ok := readCollectionChoice(w, r, "state", "online", "offline", "no_connection")
	if !ok {
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, h.directory.List(query, protocol, state))
}

func readBotAccountRef(w http.ResponseWriter, r *http.Request) (adapters.BotAccountRef, bool) {
	ref := adapters.BotAccountRef{SourceProtocol: chi.URLParam(r, "source_protocol"), BotID: chi.URLParam(r, "bot_id")}
	// chi matches RawPath when present, so opaque account IDs need the same
	// single unescape net/http already applies when only Path is used.
	if r.URL.RawPath != "" {
		var err error
		ref.SourceProtocol, err = url.PathUnescape(ref.SourceProtocol)
		if err == nil {
			ref.BotID, err = url.PathUnescape(ref.BotID)
		}
		if err != nil {
			httpapi.WriteError(w, r, errorcodes.PlatformInvalidRequest, nil)
			return ref, false
		}
	}
	if (ref.SourceProtocol != "onebot11" && ref.SourceProtocol != "qqofficial") || ref.BotID == "" {
		httpapi.WriteError(w, r, errorcodes.PlatformInvalidRequest, nil)
		return ref, false
	}
	return ref, true
}

func (h *BotAccountHandlers) get(w http.ResponseWriter, r *http.Request) {
	ref, ok := readBotAccountRef(w, r)
	if !ok {
		return
	}
	account, exists := h.directory.Get(ref)
	if !exists {
		httpapi.WriteError(w, r, errorcodes.PlatformResourceNotFound, nil)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, account)
}

func (h *BotAccountHandlers) delete(w http.ResponseWriter, r *http.Request) {
	ref, ok := readBotAccountRef(w, r)
	if !ok {
		return
	}
	err := h.directory.Delete(r.Context(), ref)
	switch {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, adapters.ErrBotAccountNotFound):
		httpapi.WriteError(w, r, errorcodes.PlatformResourceNotFound, nil)
	case errors.Is(err, adapters.ErrBotAccountConnected):
		httpapi.WriteError(w, r, errorcodes.PlatformStateConflict, nil)
	default:
		httpapi.WriteError(w, r, errorcodes.PlatformInternalError, nil)
	}
}
