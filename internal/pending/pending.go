// Package pending stages a one-click aether:// import between processes:
// the URL handler CLI writes a pending import file and launches the GUI,
// which reads and consumes it on startup.
package pending

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"aether/internal/platform"
)

// Import describes a staged web import.
type Import struct {
	SourceURL        string `json:"source_url"`
	ExternalTheme    string `json:"external_theme,omitempty"`
	ColorsToml       string `json:"colors_toml,omitempty"`
	Wallpaper        string `json:"wallpaper,omitempty"`
	Mode             string `json:"mode,omitempty"`
	Edit             bool   `json:"edit,omitempty"`
	OmarchyThemeName string `json:"omarchy_theme_name,omitempty"`
	Timestamp        int64  `json:"timestamp"`
}

// now is a test-seam for expiry checks.
var now = time.Now().Unix

// filePath is where the staged import is stored.
func filePath() string {
	return filepath.Join(platform.ConfigDir(), "pending-import.json")
}

// Read returns the staged import, or nil when none is pending. Expired
// entries (older than a day) are dropped.
func Read() (*Import, error) {
	data, err := os.ReadFile(filePath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read pending import: %w", err)
	}
	var imp Import
	if err := json.Unmarshal(data, &imp); err != nil {
		// Corrupt staging file: clear it and report nothing pending.
		_ = os.Remove(filePath())
		return nil, nil
	}
	const maxAgeSeconds = 24 * 60 * 60
	if imp.Timestamp > 0 && imp.Timestamp+maxAgeSeconds < now() {
		_ = Clear()
		return nil, nil
	}
	return &imp, nil
}

// Write stages an import, replacing any previous one.
func Write(imp *Import) error {
	if imp == nil {
		return Clear()
	}
	if imp.Timestamp == 0 {
		imp.Timestamp = now()
	}
	data, err := json.MarshalIndent(imp, "", "  ")
	if err != nil {
		return fmt.Errorf("encode pending import: %w", err)
	}
	if err := platform.EnsureDir(filepath.Dir(filePath())); err != nil {
		return err
	}
	return os.WriteFile(filePath(), data, 0o600)
}

// Clear removes the staging file.
func Clear() error {
	err := os.Remove(filePath())
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("clear pending import: %w", err)
	}
	return nil
}
