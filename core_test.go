package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstallPayload(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "addons"), 0755); err != nil {
		t.Fatal(err)
	}
	n, err := installPayload(root)
	if err != nil {
		t.Fatal(err)
	}
	if n != 7 {
		t.Fatalf("expected 7 files, got %d", n)
	}
	for _, p := range []string{"lua/autorun/server/elr_server.lua", "lua/autorun/client/elr_client.lua", "tools/Watcher.ps1", "lua/elr/web.lua"} {
		if _, err := os.Stat(filepath.Join(root, "addons", addonFolder, filepath.FromSlash(p))); err != nil {
			t.Fatal(err)
		}
	}
	n, err = installPayload(root)
	if err != nil || n != 7 {
		t.Fatalf("reinstall failed: %d %v", n, err)
	}
}
func TestCanonicalGmodFolder(t *testing.T) {
	parent := t.TempDir()
	game := filepath.Join(parent, "GarrysMod", "garrysmod")
	os.MkdirAll(filepath.Join(game, "addons"), 0755)
	for _, p := range []string{filepath.Dir(game), game, filepath.Join(game, "addons")} {
		got, err := canonicalGmodFolder(p)
		if err != nil || got != game {
			t.Fatalf("input %s => %s %v", p, got, err)
		}
	}
}

func TestResidentExecutableVersionsDoNotOverwrite(t *testing.T) {
	home := t.TempDir()
	source := filepath.Join(t.TempDir(), "release.exe")
	if err := os.WriteFile(source, []byte("ELR build A"), 0600); err != nil {
		t.Fatal(err)
	}
	first, err := installResidentExecutable(source, home)
	if err != nil {
		t.Fatal(err)
	}
	firstRepeat, err := installResidentExecutable(source, home)
	if err != nil || first != firstRepeat {
		t.Fatalf("same version must be reused: %q %q %v", first, firstRepeat, err)
	}
	if err := os.WriteFile(source, []byte("ELR build B"), 0600); err != nil {
		t.Fatal(err)
	}
	second, err := installResidentExecutable(source, home)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("different version overwrote old executable")
	}
	old, err := os.ReadFile(first)
	if err != nil || string(old) != "ELR build A" {
		t.Fatalf("original version was overwritten: %q %v", old, err)
	}
	current, err := os.ReadFile(second)
	if err != nil || string(current) != "ELR build B" {
		t.Fatalf("new version missing: %q %v", current, err)
	}
	path, err := installedResident(Settings{Resident: second}, home)
	if err != nil || path != second {
		t.Fatalf("resident resolution failed: %q %v", path, err)
	}
}

func TestResidentRejectsUnrelatedPath(t *testing.T) {
	home := t.TempDir()
	malicious := filepath.Join(t.TempDir(), "unrelated.exe")
	if err := os.WriteFile(malicious, []byte("no"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := installedResident(Settings{Resident: malicious}, home); err == nil {
		t.Fatal("unrelated path should not be launched")
	}
}
