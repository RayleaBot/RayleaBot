package deps

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (m *Manager) ResolvePreparedEntrypoint(kind, name string) (string, error) {
	prepared, err := m.resolvePreparedResource(kind)
	if err != nil {
		if kind == "chromium" && name == "browser" {
			return m.resolveSystemChromiumEntrypoint(context.Background())
		}
		return "", err
	}
	path, ok := prepared.Entrypoints[name]
	if !ok {
		return "", fmt.Errorf("entrypoint %s is not declared for %s", name, kind)
	}
	return path, nil
}

func (m *Manager) ResolveEntrypoint(ctx context.Context, kind, name string) (string, error) {
	prepared, err := m.Prepare(ctx, kind)
	if err != nil {
		return "", err
	}
	path, ok := prepared.Entrypoints[name]
	if !ok {
		return "", fmt.Errorf("entrypoint %s is not declared for %s", name, kind)
	}
	return path, nil
}

func (m *Manager) resolveSystemChromiumEntrypoint(ctx context.Context) (string, error) {
	if m == nil || m.findSystemChromium == nil {
		return "", errSystemChromiumUnavailable
	}
	path, err := m.findSystemChromium(ctx)
	if err != nil {
		return "", err
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return "", errSystemChromiumUnavailable
	}
	return path, nil
}

func systemChromiumPreparedResource(path string) *PreparedResource {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	resource := Resource{
		ID:       "system-chromium",
		Kind:     "chromium",
		Version:  "system",
		Platform: CurrentPlatform(),
	}
	return &PreparedResource{
		Resource: resource,
		Root:     filepath.Dir(path),
		Entrypoints: map[string]string{
			"browser": path,
		},
	}
}

func (m *Manager) resolvePreparedResource(kind string) (*PreparedResource, error) {
	manifest, resource, err := m.currentResource(kind)
	if err != nil {
		if kind == "chromium" {
			if path, pathErr := m.resolveSystemChromiumEntrypoint(context.Background()); pathErr == nil {
				return systemChromiumPreparedResource(path), nil
			}
		}
		return nil, err
	}
	prepared, err := m.resolvePreparedManifestResource(manifest, resource)
	if err == nil {
		return prepared, nil
	}
	if kind == "chromium" {
		if path, pathErr := m.resolveSystemChromiumEntrypoint(context.Background()); pathErr == nil {
			return systemChromiumPreparedResource(path), nil
		}
	}
	return nil, err
}

func (m *Manager) resolvePreparedManifestResource(_ *Manifest, resource *Resource) (*PreparedResource, error) {
	storeRoot := StoreRoot(m.repoRoot, resource)
	entrypoints, err := resolvePreparedEntrypoints(storeRoot, resource)
	if err != nil {
		return nil, err
	}
	return &PreparedResource{
		Resource:    *resource,
		Root:        storeRoot,
		Entrypoints: entrypoints,
	}, nil
}

func (m *Manager) currentResource(kind string) (*Manifest, *Resource, error) {
	manifest, err := LoadManifest(m.repoRoot)
	if err != nil {
		return nil, nil, err
	}
	resource := manifest.FindResource(CurrentPlatform(), kind)
	if resource == nil {
		return manifest, nil, fmt.Errorf("deps manifest does not include %s for %s", kind, CurrentPlatform())
	}
	return manifest, resource, nil
}

func resolvePreparedEntrypoints(storeRoot string, resource *Resource) (map[string]string, error) {
	if resource == nil {
		return nil, errors.New("deps resource is required")
	}
	entrypoints := make(map[string]string, len(resource.Entrypoints))
	for _, key := range requiredEntrypoints(resource) {
		candidates := resource.Entrypoints[key]
		var resolved string
		for _, candidate := range candidates {
			clean := filepath.Clean(filepath.Join(storeRoot, filepath.FromSlash(candidate)))
			if !pathWithinRoot(storeRoot, clean) {
				continue
			}
			info, err := os.Stat(clean)
			if err != nil || info.IsDir() {
				continue
			}
			resolved = clean
			break
		}
		if resolved == "" {
			return nil, fmt.Errorf("prepared deps resource %s is missing entrypoint %s", resource.Kind, key)
		}
		entrypoints[key] = resolved
	}
	return entrypoints, nil
}

func primaryEntrypoint(prepared *PreparedResource) string {
	if prepared == nil {
		return ""
	}
	for _, key := range requiredEntrypoints(&prepared.Resource) {
		if entry := strings.TrimSpace(prepared.Entrypoints[key]); entry != "" {
			return entry
		}
	}
	return ""
}
