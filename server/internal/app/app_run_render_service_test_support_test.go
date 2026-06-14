package app

import (
	"context"
	"encoding/base64"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/render"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

var (
	testRenderPNGBytes, _  = base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO2W4n8AAAAASUVORK5CYII=")
	testRenderJPEGBytes, _ = base64.StdEncoding.DecodeString("/9j/4AAQSkZJRgABAQAAAQABAAD/2wCEAAkGBxAQEBAQEA8PDw8PDw8PDw8PDw8PDw8QFREWFhURFRUYHSggGBolGxUVITEhJSkrLi4uFx8zODMsNygtLisBCgoKDg0OGxAQGy0lICYtLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLf/AABEIAAEAAQMBEQACEQEDEQH/xAAXAAEBAQEAAAAAAAAAAAAAAAAAAQID/8QAFBABAAAAAAAAAAAAAAAAAAAAAP/aAAwDAQACEAMQAAAB6gD/xAAXEAEBAQEAAAAAAAAAAAAAAAABEQAh/9oACAEBAAEFAjQ2qf/EABQRAQAAAAAAAAAAAAAAAAAAABD/2gAIAQMBAT8BP//EABQRAQAAAAAAAAAAAAAAAAAAABD/2gAIAQIBAT8BP//EABYQAQEBAAAAAAAAAAAAAAAAAAERIf/aAAgBAQAGPwIhZ//EABgQAQEBAQEAAAAAAAAAAAAAAAERACEx/9oACAEBAAE/IZmBliTFkY2l/9oADAMBAAIAAwAAABAP/8QAFBEBAAAAAAAAAAAAAAAAAAAAEP/aAAgBAwEBPxA//8QAFBEBAAAAAAAAAAAAAAAAAAAAEP/aAAgBAgEBPxA//8QAGBABAAMBAAAAAAAAAAAAAAAAAQARITFR/9oACAEBAAE/EKQhNQIfY0x0KGLX/9k=")
)

type staticRenderRunner struct{}

func (staticRenderRunner) Render(_ context.Context, doc render.Document) ([]byte, error) {
	if doc.Output == "jpeg" {
		return append([]byte(nil), testRenderJPEGBytes...), nil
	}
	return append([]byte(nil), testRenderPNGBytes...), nil
}

type captureRenderRunner struct {
	mu   sync.Mutex
	docs []render.Document
}

func (r *captureRenderRunner) Render(_ context.Context, doc render.Document) ([]byte, error) {
	r.mu.Lock()
	r.docs = append(r.docs, doc)
	r.mu.Unlock()

	if doc.Output == "jpeg" {
		return append([]byte(nil), testRenderJPEGBytes...), nil
	}
	return append([]byte(nil), testRenderPNGBytes...), nil
}

func (r *captureRenderRunner) lastHTML() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.docs) == 0 {
		return ""
	}
	return r.docs[len(r.docs)-1].HTML
}

func newRenderService(t *testing.T, root string) *render.Service {
	t.Helper()

	repoRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		panic(err)
	}
	return newRenderServiceForRepo(t, repoRoot, root, staticRenderRunner{})
}

func newRenderServiceForRepo(t *testing.T, repoRoot string, root string, runner render.Runner) *render.Service {
	t.Helper()

	store, err := storage.Open(filepath.Join(root, "render-state.db"))
	if err != nil {
		panic(err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})

	service, err := render.NewService(render.Options{
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
