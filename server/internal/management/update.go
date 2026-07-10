package management

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/releaseupdate"
)

type UpdateService interface {
	Status() releaseupdate.StatusSnapshot
	Check(context.Context) (releaseupdate.StatusSnapshot, error)
}

type UpdateHandlers struct {
	service UpdateService
}

type updateStatusResponse struct {
	State                     string     `json:"state"`
	Phase                     string     `json:"phase,omitempty"`
	CurrentVersion            string     `json:"current_version"`
	AvailableVersion          string     `json:"available_version,omitempty"`
	CheckedAt                 *time.Time `json:"checked_at"`
	UpdateMode                string     `json:"update_mode"`
	AutomaticInstallSupported bool       `json:"automatic_install_supported"`
	ReleaseNotesRef           string     `json:"release_notes_ref,omitempty"`
}

func NewUpdateHandlers(service UpdateService) *UpdateHandlers {
	return &UpdateHandlers{service: service}
}

func (h *UpdateHandlers) RegisterProtectedRoutes(router chi.Router) {
	router.Get("/api/update/status", h.HandleStatus())
	router.Post("/api/update/check", h.HandleCheck())
}

func (h *UpdateHandlers) HandleStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if h == nil || h.service == nil {
			httpapi.WriteJSON(w, http.StatusOK, responseFromUpdateSnapshot(releaseupdate.StatusSnapshot{
				State:          "disabled",
				CurrentVersion: "unknown",
				UpdateMode:     "unavailable",
			}))
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, responseFromUpdateSnapshot(h.service.Status()))
	}
}

func (h *UpdateHandlers) HandleCheck() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h == nil || h.service == nil {
			httpapi.WriteError(w, r, http.StatusConflict, releaseupdate.CodeTrustRequired, "当前安装不具备受信任更新基线", "errors.release.trust_required", nil)
			return
		}
		snapshot, err := h.service.Check(r.Context())
		if err != nil {
			if errors.Is(err, releaseupdate.ErrCheckInProgress) {
				httpapi.WriteError(w, r, http.StatusTooManyRequests, systemCodeTaskQueueFull, "更新检查正在进行，请稍后重试", "errors.platform.task_queue_full", nil)
				return
			}
			code := releaseupdate.CodeOf(err)
			if code == "" {
				code = releaseupdate.CodeManifestInvalid
			}
			httpapi.WriteError(w, r, http.StatusConflict, code, "无法确认受信任的更新", "errors."+code, nil)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, responseFromUpdateSnapshot(snapshot))
	}
}

func responseFromUpdateSnapshot(snapshot releaseupdate.StatusSnapshot) updateStatusResponse {
	return updateStatusResponse{
		State:                     snapshot.State,
		Phase:                     string(snapshot.Phase),
		CurrentVersion:            snapshot.CurrentVersion,
		AvailableVersion:          snapshot.AvailableVersion,
		CheckedAt:                 snapshot.CheckedAt,
		UpdateMode:                snapshot.UpdateMode,
		AutomaticInstallSupported: snapshot.AutomaticInstallSupported,
		ReleaseNotesRef:           snapshot.ReleaseNotesRef,
	}
}
