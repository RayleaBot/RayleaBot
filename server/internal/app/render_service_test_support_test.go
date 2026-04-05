package app

import (
	"context"
	"encoding/base64"
	"path/filepath"
	"testing"
	"time"

	"rayleabot/server/internal/render"
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

func newRenderService(root string) *render.Service {
	repoRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		panic(err)
	}

	service, err := render.NewService(render.Options{
		RepoRoot:           repoRoot,
		OutputRoot:         root,
		Runner:             staticRenderRunner{},
		WorkerCount:        1,
		QueueMaxLength:     2,
		QueueWaitTimeout:   time.Second,
		RenderTimeout:      time.Second,
		MaxRenderDataBytes: 1 << 20,
	})
	if err != nil {
		panic(err)
	}
	return service
}
