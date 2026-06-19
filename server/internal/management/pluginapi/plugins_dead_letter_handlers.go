package pluginapi

import (
	"net/http"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/go-chi/chi/v5"
)

func registerPluginDeadLetterRoutes(router chi.Router, catalog plugins.CatalogView, controller DesiredStateController, grantRepo plugins.GrantRepository, autoGrantProvider autoGrantCapabilitiesProvider) {
	router.Post("/api/plugins/{plugin_id}/dead_letter/recover", newDeadLetterRecoverHandler(catalog, controller, grantRepo, autoGrantProvider))
}

func newDeadLetterRecoverHandler(catalog plugins.CatalogView, controller DesiredStateController, grantRepo plugins.GrantRepository, autoGrantProvider autoGrantCapabilitiesProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pluginID := chi.URLParam(r, "plugin_id")
		if controller == nil {
			writeError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
			return
		}
		snapshot, err := controller.RecoverFromDeadLetter(r.Context(), pluginID)
		if err == nil {
			writePluginDetailResponse(w, r, catalog, snapshot, grantRepo, autoGrantProvider)
			return
		}
		writeDesiredStateError(w, r, pluginID, err)
	}
}
