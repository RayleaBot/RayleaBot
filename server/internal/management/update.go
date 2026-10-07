package management

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/releaseupdate"
)

type UpdateService interface {
	Releases(context.Context) ([]releaseupdate.Release, error)
	Status() releaseupdate.StatusSnapshot
	Check(context.Context) (releaseupdate.StatusSnapshot, error)
}

type UpdateHandlers struct {
	service UpdateService
}

type updateStatusResponse struct {
	Routes           []releaseupdate.Route `json:"routes,omitempty"`
	SourceURL        string                `json:"source_url,omitempty"`
	State            string                `json:"state"`
	CurrentVersion   string                `json:"current_version"`
	AvailableVersion string                `json:"available_version,omitempty"`
	CheckedAt        *time.Time            `json:"checked_at"`
	UpdateMode       string                `json:"update_mode"`
	ReleaseNotesRef  string                `json:"release_notes_ref,omitempty"`
}

func NewUpdateHandlers(service UpdateService) (*UpdateHandlers, error) {
	if service == nil {
		return nil, errors.New("update service is required")
	}
	return &UpdateHandlers{service: service}, nil
}

func (h *UpdateHandlers) RegisterProtectedRoutes(router chi.Router) {
	router.Get("/api/update/status", h.HandleStatus())
	router.Post("/api/update/check", h.HandleCheck())
	router.Get("/api/update/releases", h.HandleReleases())
}

func (h *UpdateHandlers) HandleReleases() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		releases, err := h.service.Releases(r.Context())
		if err != nil {
			httpapi.WriteError(w, r, releaseupdate.CodeManifestInvalid, nil)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, struct {
			Releases []releaseupdate.Release `json:"releases"`
		}{releases})
	}
}

func (h *UpdateHandlers) HandleStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		httpapi.WriteJSON(w, http.StatusOK, responseFromUpdateSnapshot(h.service.Status()))
	}
}

func (h *UpdateHandlers) HandleCheck() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		snapshot, err := h.service.Check(r.Context())
		if err != nil {
			if errors.Is(err, releaseupdate.ErrCheckInProgress) {
				httpapi.WriteError(w, r, systemCodeTaskQueueFull, nil)
				return
			}
			code := releaseupdate.CodeOf(err)
			if code == "" {
				code = releaseupdate.CodeManifestInvalid
			}
			httpapi.WriteError(w, r, code, nil)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, responseFromUpdateSnapshot(snapshot))
	}
}

func responseFromUpdateSnapshot(snapshot releaseupdate.StatusSnapshot) updateStatusResponse {
	return updateStatusResponse{
		Routes:           snapshot.Routes,
		SourceURL:        snapshot.SourceURL,
		State:            snapshot.State,
		CurrentVersion:   snapshot.CurrentVersion,
		AvailableVersion: snapshot.AvailableVersion,
		CheckedAt:        snapshot.CheckedAt,
		UpdateMode:       snapshot.UpdateMode,
		ReleaseNotesRef:  snapshot.ReleaseNotesRef,
	}
}
