package testutil

import (
	"context"
	"encoding/base64"
	"path/filepath"
	"sync"
	"testing"
	"time"

	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

var (
	RenderPNGBytes, _  = base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO2W4n8AAAAASUVORK5CYII=")
	RenderJPEGBytes, _ = base64.StdEncoding.DecodeString("/9j/4AAQSkZJRgABAQAAAQABAAD/2wCEAAkGBxAQEBAQEA8PDw8PDw8PDw8PDw8PDw8QFREWFhURFRUYHSggGBolGxUVITEhJSkrLi4uFx8zODMsNygtLisBCgoKDg0OGxAQGy0lICYtLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLf/AABEIAAEAAQMBEQACEQEDEQH/xAAXAAEBAQEAAAAAAAAAAAAAAAAAAQID/8QAFBABAAAAAAAAAAAAAAAAAAAAAP/aAAwDAQACEAMQAAAB6gD/xAAXEAEBAQEAAAAAAAAAAAAAAAABEQAh/9oACAEBAAEFAjQ2qf/EABQRAQAAAAAAAAAAAAAAAAAAABD/2gAIAQMBAT8BP//EABQRAQAAAAAAAAAAAAAAAAAAABD/2gAIAQIBAT8BP//EABYQAQEBAAAAAAAAAAAAAAAAAAERIf/aAAgBAQAGPwIhZ//EABgQAQEBAQEAAAAAAAAAAAAAAAERACEx/9oACAEBAAE/IZmBliTFkY2l/9oADAMBAAIAAwAAABAP/8QAFBEBAAAAAAAAAAAAAAAAAAAAEP/aAAgBAwEBPxA//8QAFBEBAAAAAAAAAAAAAAAAAAAAEP/aAAgBAgEBPxA//8QAGBABAAMBAAAAAAAAAAAAAAAAAQARITFR/9oACAEBAAE/EKQhNQIfY0x0KGLX/9k=")
)

type StaticRenderRunner struct{}

func (StaticRenderRunner) Render(_ context.Context, doc renderservice.Document) ([]byte, error) {
	if doc.Output == "jpeg" {
		return append([]byte(nil), RenderJPEGBytes...), nil
	}
	return append([]byte(nil), RenderPNGBytes...), nil
}

type CaptureRenderRunner struct {
	mu   sync.Mutex
	docs []renderservice.Document
}

func (r *CaptureRenderRunner) Render(_ context.Context, doc renderservice.Document) ([]byte, error) {
	r.mu.Lock()
	r.docs = append(r.docs, doc)
	r.mu.Unlock()

	if doc.Output == "jpeg" {
		return append([]byte(nil), RenderJPEGBytes...), nil
	}
	return append([]byte(nil), RenderPNGBytes...), nil
}

func (r *CaptureRenderRunner) LastHTML() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.docs) == 0 {
		return ""
	}
	return r.docs[len(r.docs)-1].HTML
}

func NewRenderService(t *testing.T, root string) *renderservice.Service {
	t.Helper()

	repoRoot := RepoRoot(t)
	return NewRenderServiceForRepo(t, repoRoot, root, StaticRenderRunner{})
}

func NewRenderServiceForRepo(t *testing.T, repoRoot string, root string, runner renderservice.Runner) *renderservice.Service {
	t.Helper()

	store, err := storage.Open(filepath.Join(root, "render-state.db"))
	if err != nil {
		panic(err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})

	service, err := renderservice.NewService(renderservice.Options{
		RepoRoot:           repoRoot,
		OutputRoot:         root,
		Store:              store,
		Runner:             runner,
		WorkerCount:        1,
		QueueMaxLength:     2,
		QueueWaitTimeout:   time.Second,
		RenderTimeout:      time.Second,
		MaxRenderDataBytes: 1 << 20,
	})
	if err != nil {
		panic(err)
	}
	t.Cleanup(func() {
		_ = service.Close()
	})
	return service
}
