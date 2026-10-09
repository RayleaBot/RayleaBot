//go:build linux

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRegisterLauncherDesktop(t *testing.T) {
	dataHome := t.TempDir()
	executable := `/home/user/下载/space "quote" $cash ` + "`tick`" + ` \slash 100%/RayleaLauncher`
	icon := []byte("embedded icon")
	if err := registerLauncherDesktop(dataHome, executable, icon); err != nil {
		t.Fatal(err)
	}
	entryPath := filepath.Join(dataHome, "applications", launcherApplicationID+".desktop")
	entry, err := os.ReadFile(entryPath)
	if err != nil {
		t.Fatal(err)
	}
	wantExec := `Exec="/home/user/下载/space \\"quote\\" \\$cash ` + "\\\\`tick\\\\`" + ` \\\\slash 100%%/RayleaLauncher"` + "\n"
	if !strings.Contains(string(entry), wantExec) {
		t.Fatalf("Exec must escape desktop entry and command arguments:\n%s\nwant %s", entry, wantExec)
	}
	writtenIcon, err := os.ReadFile(filepath.Join(dataHome, "rayleabot", "launcher-icon.png"))
	if err != nil || !bytes.Equal(writtenIcon, icon) {
		t.Fatalf("registered icon differs from embedded icon: %v", err)
	}
	before, _ := os.Stat(entryPath)
	if err := registerLauncherDesktop(dataHome, executable, icon); err != nil {
		t.Fatal(err)
	}
	after, _ := os.Stat(entryPath)
	if !os.SameFile(before, after) {
		t.Fatal("unchanged desktop entry was unnecessarily replaced")
	}
	if err := registerLauncherDesktop(dataHome, "/moved/RayleaLauncher", icon); err != nil {
		t.Fatal(err)
	}
	updated, _ := os.ReadFile(entryPath)
	if !strings.Contains(string(updated), "Exec=\"/moved/RayleaLauncher\"\n") {
		t.Fatal("desktop association did not follow the moved bundle")
	}
}

func TestPrepareDesktopIntegrationDataHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	for _, value := range []string{"", "relative", filepath.Join(home, "custom-data")} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("XDG_DATA_HOME", value)
			if err := prepareDesktopIntegration([]byte("icon")); err != nil {
				t.Fatal(err)
			}
			dataHome := value
			if !filepath.IsAbs(value) {
				dataHome = filepath.Join(home, ".local", "share")
			}
			if _, err := os.Stat(filepath.Join(dataHome, "applications", launcherApplicationID+".desktop")); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRegisterLauncherDesktopRejectsInvalidPathsAndWriteFailure(t *testing.T) {
	for _, executable := range []string{"relative", "/tmp/a=b/Launcher", "/tmp/zero\x00/Launcher"} {
		if err := registerLauncherDesktop(t.TempDir(), executable, []byte("icon")); err == nil {
			t.Fatalf("accepted unsupported executable %q", executable)
		}
	}
	dataHome := t.TempDir()
	if err := os.WriteFile(filepath.Join(dataHome, "rayleabot"), []byte("blocking file"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := registerLauncherDesktop(dataHome, "/tmp/Launcher", []byte("icon")); err == nil {
		t.Fatal("registration silently ignored an icon write failure")
	}
	if _, err := os.Stat(filepath.Join(dataHome, "applications", launcherApplicationID+".desktop")); !os.IsNotExist(err) {
		t.Fatal("desktop entry was published without its icon")
	}
}
