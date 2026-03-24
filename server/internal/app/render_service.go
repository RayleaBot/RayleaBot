package app

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	placeholderPNGBytes, _  = base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO2W4n8AAAAASUVORK5CYII=")
	placeholderJPEGBytes, _ = base64.StdEncoding.DecodeString("/9j/4AAQSkZJRgABAQAAAQABAAD/2wCEAAkGBxAQEBAQEA8PDw8PDw8PDw8PDw8PDw8QFREWFhURFRUYHSggGBolGxUVITEhJSkrLi4uFx8zODMsNygtLisBCgoKDg0OGxAQGy0lICYtLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLf/AABEIAAEAAQMBEQACEQEDEQH/xAAXAAEBAQEAAAAAAAAAAAAAAAAAAQID/8QAFBABAAAAAAAAAAAAAAAAAAAAAP/aAAwDAQACEAMQAAAB6gD/xAAXEAEBAQEAAAAAAAAAAAAAAAABEQAh/9oACAEBAAEFAjQ2qf/EABQRAQAAAAAAAAAAAAAAAAAAABD/2gAIAQMBAT8BP//EABQRAQAAAAAAAAAAAAAAAAAAABD/2gAIAQIBAT8BP//EABYQAQEBAAAAAAAAAAAAAAAAAAERIf/aAAgBAQAGPwIhZ//EABgQAQEBAQEAAAAAAAAAAAAAAAERACEx/9oACAEBAAE/IZmBliTFkY2l/9oADAMBAAIAAwAAABAP/8QAFBEBAAAAAAAAAAAAAAAAAAAAEP/aAAgBAwEBPxA//8QAFBEBAAAAAAAAAAAAAAAAAAAAEP/aAAgBAgEBPxA//8QAGBABAAMBAAAAAAAAAAAAAAAAAQARITFR/9oACAEBAAE/EKQhNQIfY0x0KGLX/9k=")
)

type renderService struct {
	root string
	mu   sync.Mutex
}

type renderResult struct {
	ImagePath    string
	MIME         string
	CacheKey     string
	FallbackSent bool
}

func newRenderService(root string) *renderService {
	return &renderService{root: root}
}

func (s *renderService) Render(templateName, theme, output string, data map[string]any) (renderResult, error) {
	if s == nil {
		return renderResult{}, fmt.Errorf("render service is not available")
	}

	if strings.TrimSpace(theme) == "" {
		theme = "default"
	}
	if strings.TrimSpace(output) == "" {
		output = "png"
	}

	raw, err := json.Marshal(data)
	if err != nil {
		return renderResult{}, fmt.Errorf("marshal render payload: %w", err)
	}

	sum := sha1.Sum(append([]byte(templateName+":"+theme+":"), raw...))
	cacheDigest := hex.EncodeToString(sum[:])[:8]
	cacheKey := fmt.Sprintf("%s:%s:%s", templateName, theme, cacheDigest)

	filename := sanitizeRenderTemplate(templateName) + "-" + cacheDigest + "." + output
	if err := os.MkdirAll(s.root, 0o755); err != nil {
		return renderResult{}, fmt.Errorf("create render directory: %w", err)
	}

	targetPath := filepath.Join(s.root, filename)

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		content, mimeType := renderPlaceholderArtifact(output)
		if err := os.WriteFile(targetPath, content, 0o644); err != nil {
			return renderResult{}, fmt.Errorf("write render artifact: %w", err)
		}
		return renderResult{
			ImagePath:    fileURL(targetPath),
			MIME:         mimeType,
			CacheKey:     cacheKey,
			FallbackSent: false,
		}, nil
	} else if err != nil {
		return renderResult{}, fmt.Errorf("stat render artifact: %w", err)
	}

	_, mimeType := renderPlaceholderArtifact(output)
	return renderResult{
		ImagePath:    fileURL(targetPath),
		MIME:         mimeType,
		CacheKey:     cacheKey,
		FallbackSent: false,
	}, nil
}

func renderPlaceholderArtifact(output string) ([]byte, string) {
	switch strings.ToLower(strings.TrimSpace(output)) {
	case "jpeg":
		return append([]byte(nil), placeholderJPEGBytes...), "image/jpeg"
	default:
		return append([]byte(nil), placeholderPNGBytes...), "image/png"
	}
}

func sanitizeRenderTemplate(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "/", "-")
	value = strings.ReplaceAll(value, "\\", "-")
	value = strings.ReplaceAll(value, ":", "-")
	if value == "" {
		return "render"
	}
	return value
}

func fileURL(path string) string {
	return (&url.URL{
		Scheme: "file",
		Path:   filepath.ToSlash(path),
	}).String()
}
