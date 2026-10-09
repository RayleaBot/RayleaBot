//go:build windows

package render

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestTemplateAssetsRejectWindowsJunctionEscapesAndSourceAliases(t *testing.T) {
	repoRoot, outside := t.TempDir(), t.TempDir()
	templatesRoot := filepath.Join(repoRoot, "templates")
	writeRenderTemplateSeed(t, templatesRoot, "card")
	if err := os.WriteFile(filepath.Join(outside, "private.txt"), []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	cardRoot := filepath.Join(templatesRoot, "card")
	for name, target := range map[string]string{"outside": outside, "source": cardRoot} {
		command := exec.Command("cmd", "/c", "mklink", "/J", filepath.Join(cardRoot, name), target)
		command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("create junction: %v %s", err, output)
		}
	}
	service, err := NewService(Options{RepoRoot: repoRoot, OutputRoot: filepath.Join(repoRoot, "output"), Store: openRenderTestStore(t), Runner: &fakeRunner{}, WorkerCount: 1, QueueMaxLength: 2, QueueWaitTimeout: time.Second, RenderTimeout: time.Second, MaxRenderDataBytes: 256 * 1024})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close() })
	for _, name := range []string{"outside/private.txt", "source/template.html"} {
		if _, err := service.LookupTemplateAsset(t.Context(), "card", name); err == nil {
			t.Fatalf("served junction %s", name)
		}
	}
	asset := TemplateAsset{ResourceRoot: templatesRoot, Path: filepath.Join(cardRoot, "outside", "private.txt")}
	if file, err := asset.Open(); err == nil {
		_ = file.Close()
		t.Fatal("opened junction outside resource root")
	}
}
