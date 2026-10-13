package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

//go:embed payload.zip
var originalELR []byte

const addonFolder = "extreme_lua_refresh"

type Settings struct {
	GameDir  string `json:"game_dir"`
	Language string `json:"language"`
	Auto     bool   `json:"auto"`
	Resident string `json:"resident_exe,omitempty"`
}

type MonitorStatus struct {
	Mode      string `json:"mode"`
	Game      bool   `json:"game"`
	Watcher   bool   `json:"watcher"`
	PID       int    `json:"pid"`
	Timestamp int64  `json:"timestamp"`
	Error     string `json:"error,omitempty"`
}

func canonicalGmodFolder(given string) (string, error) {
	g := strings.TrimSpace(strings.Trim(given, "\""))
	if g == "" {
		return "", errors.New("sélectionne le répertoire Garry's Mod")
	}
	abs, err := filepath.Abs(g)
	if err != nil {
		return "", err
	}
	abs = filepath.Clean(abs)
	switch strings.ToLower(filepath.Base(abs)) {
	case "addons":
		abs = filepath.Dir(abs)
	case "garrysmod":
		if st, err := os.Stat(filepath.Join(abs, "garrysmod")); err == nil && st.IsDir() {
			abs = filepath.Join(abs, "garrysmod")
		}
	}
	stat, err := os.Stat(abs)
	if err != nil || !stat.IsDir() {
		return "", fmt.Errorf("répertoire introuvable : %s", abs)
	}
	if st, err := os.Stat(filepath.Join(abs, "addons")); err != nil || !st.IsDir() {
		return "", fmt.Errorf("dossier addons introuvable dans %s", abs)
	}
	return abs, nil
}

func installPayload(root string) (int, error) {
	r, err := zip.NewReader(bytes.NewReader(originalELR), int64(len(originalELR)))
	if err != nil {
		return 0, err
	}
	addons := filepath.Join(root, "addons")
	n := 0
	for _, f := range r.File {
		name := filepath.FromSlash(f.Name)
		cleaned := filepath.Clean(name)
		// No absolute paths, parent traversal, or writes outside the packaged addon.
		if filepath.IsAbs(cleaned) || strings.HasPrefix(cleaned, ".."+string(os.PathSeparator)) || cleaned == ".." || !(cleaned == addonFolder || strings.HasPrefix(cleaned, addonFolder+string(os.PathSeparator))) {
			return n, fmt.Errorf("chemin invalide dans l'archive : %s", name)
		}
		dest := filepath.Join(addons, cleaned)
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(dest, 0755); err != nil {
				return n, err
			}
			continue
		}
		if f.Mode()&os.ModeSymlink != 0 {
			return n, fmt.Errorf("lien symbolique non supporté : %s", name)
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			return n, err
		}
		src, err := f.Open()
		if err != nil {
			return n, err
		}
		tmp, err := os.CreateTemp(filepath.Dir(dest), ".elr-*")
		if err != nil {
			src.Close()
			return n, err
		}
		_, copyErr := io.Copy(tmp, src)
		src.Close()
		closeErr := tmp.Close()
		if copyErr != nil || closeErr != nil {
			os.Remove(tmp.Name())
			if copyErr != nil {
				return n, copyErr
			}
			return n, closeErr
		}
		// Windows rename does not replace an existing file; remove only the known file.
		if err := os.Remove(dest); err != nil && !os.IsNotExist(err) {
			os.Remove(tmp.Name())
			return n, err
		}
		if err := os.Rename(tmp.Name(), dest); err != nil {
			os.Remove(tmp.Name())
			return n, err
		}
		n++
	}
	if n < 4 {
		return n, errors.New("archive embarquée incomplète")
	}
	return n, nil
}

// installResidentExecutable stores each app build under a content-derived filename.
// Older running binaries are deliberately left untouched: Windows locks running EXEs.
func installResidentExecutable(sourcePath, home string) (string, error) {
	source, err := os.Open(sourcePath)
	if err != nil {
		return "", fmt.Errorf("open executable: %w", err)
	}
	defer source.Close()
	digest := sha256.New()
	if _, err = io.Copy(digest, source); err != nil {
		return "", fmt.Errorf("hash executable: %w", err)
	}
	name := "ExtremeLuaRefresh_" + hex.EncodeToString(digest.Sum(nil)[:12]) + ".exe"
	destinationDir := filepath.Join(home, "versions")
	if err := os.MkdirAll(destinationDir, 0755); err != nil {
		return "", err
	}
	destination := filepath.Join(destinationDir, name)
	if same, err := filesEqualHash(destination, digest.Sum(nil)); err != nil {
		return "", err
	} else if same {
		return destination, nil
	}
	if _, err := os.Stat(destination); err == nil {
		return "", fmt.Errorf("existing version has unexpected contents: %s", destination)
	} else if !os.IsNotExist(err) {
		return "", err
	}
	if _, err := source.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(destinationDir, ".elr-version-*.tmp")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	if _, err := io.Copy(tmp, source); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	// Target is content-addressed, so no running executable is overwritten.
	if err := os.Rename(tmp.Name(), destination); err != nil {
		if same, checkErr := filesEqualHash(destination, digest.Sum(nil)); checkErr == nil && same {
			return destination, nil // another installer placed exactly the same version
		}
		return "", err
	}
	return destination, nil
}

func filesEqualHash(path string, want []byte) (bool, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return false, err
	}
	return bytes.Equal(h.Sum(nil), want), nil
}

func installedResident(s Settings, home string) (string, error) {
	if s.Resident != "" {
		p := filepath.Clean(s.Resident)
		allowed := filepath.Join(home, "versions")
		rel, err := filepath.Rel(allowed, p)
		if err != nil || rel == "." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || rel == ".." || filepath.IsAbs(rel) || filepath.Dir(rel) != "." || !strings.HasPrefix(filepath.Base(p), "ExtremeLuaRefresh_") || !strings.EqualFold(filepath.Ext(p), ".exe") {
			return "", fmt.Errorf("invalid installed monitor path")
		}
		if st, err := os.Stat(p); err != nil || st.IsDir() {
			return "", fmt.Errorf("installed monitor not found: %s", p)
		}
		return p, nil
	}
	// Compatibility with existing 1.0.1 / 1.2.0 installations.
	legacy := filepath.Join(home, "ExtremeLuaRefresh.exe")
	if st, err := os.Stat(legacy); err == nil && !st.IsDir() {
		return legacy, nil
	}
	return "", fmt.Errorf("install the application first")
}

func loadSettings(path string) Settings {
	out := Settings{Language: "fr"}
	b, err := os.ReadFile(path)
	if err == nil {
		_ = json.Unmarshal(b, &out)
	}
	if out.Language != "en" {
		out.Language = "fr"
	}
	return out
}
func saveJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0600); err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}
func readStatus(path string) MonitorStatus {
	s := MonitorStatus{Mode: "offline"}
	b, err := os.ReadFile(path)
	if err == nil {
		_ = json.Unmarshal(b, &s)
	}
	if time.Now().Unix()-s.Timestamp > 7 {
		s.Mode = "offline"
		s.Game = false
		s.Watcher = false
	}
	return s
}
