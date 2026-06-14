package systemapi

import "net/http"

func (h *SystemHandlers) HandleSystemDiagnosticsExport() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h == nil || h.system == nil {
			writeAppError(w, r, http.StatusInternalServerError, codeInternalError, "内部错误", "errors.platform.internal_error", nil)
			return
		}
		archive, err := h.system.BuildDiagnosticsArchive(r.Context())
		if err != nil {
			writeAppError(w, r, http.StatusInternalServerError, codeInternalError, "内部错误", "errors.platform.internal_error", nil)
			return
		}

		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", `attachment; filename="rayleabot-diagnostics.zip"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(archive)
	}
}
