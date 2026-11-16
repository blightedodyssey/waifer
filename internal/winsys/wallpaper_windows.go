//go:build windows

package winsys

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

// backupDir is where winsys stores pre-apply personalization snapshots.
func backupDir() string {
	return filepath.Join(appDataDir(), "aether")
}

// backupPath is the JSON file holding the last backup.
func backupPath() string {
	return filepath.Join(backupDir(), "winsys-backup.json")
}

// personalizationBackup captures the accent color and wallpaper in effect
// before Aether changed them.
type personalizationBackup struct {
	ColorizationColor *uint32 `json:"colorizationColor,omitempty"`
	Wallpaper         string  `json:"wallpaper,omitempty"`
}

// SetWallpaper applies a wallpaper via SystemParametersInfoW and records a
// backup of the previous value.
func SetWallpaper(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve wallpaper path: %w", err)
	}
	if _, err := os.Stat(abs); err != nil {
		return fmt.Errorf("wallpaper file: %w", err)
	}

	previous := managedWallpaperPath()
	if err := saveWallpaperBackup(abs, previous); err != nil {
		return err
	}

	p := u16Ptr(abs)
	if p == nil {
		return fmt.Errorf("wallpaper path encoding failed")
	}
	if err := systemParametersInfoW(spiSetDeskWallpaper, 0, p); err != nil {
		return fmt.Errorf("set wallpaper: %w", err)
	}
	return nil
}

// managedWallpaperPath reads the current desktop wallpaper from the
// registry (Control Panel\Desktop\Wallpaper style value is not the path;
// the TranscodedImageCache or Wallpaper value is used).
func managedWallpaperPath() string {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Control Panel\Desktop`, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer key.Close()
	value, _, err := key.GetStringValue("Wallpaper")
	if err != nil {
		return ""
	}
	return value
}

// saveWallpaperBackup persists the previous wallpaper once.
func saveWallpaperBackup(current, previous string) error {
	if err := os.MkdirAll(backupDir(), 0o755); err != nil {
		return fmt.Errorf("create backup dir: %w", err)
	}
	var backup personalizationBackup
	if data, err := os.ReadFile(backupPath()); err == nil {
		_ = json.Unmarshal(data, &backup)
	}
	if backup.Wallpaper == "" {
		backup.Wallpaper = previous
	}
	_ = current
	data, err := json.MarshalIndent(backup, "", "  ")
	if err != nil {
		return fmt.Errorf("encode backup: %w", err)
	}
	return writeFileAtomic(backupPath(), string(data)+"\n")
}

// restoreBackup reverts the wallpaper captured before the first apply.
func restoreBackup() error {
	var backup personalizationBackup
	data, err := os.ReadFile(backupPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if err := json.Unmarshal(data, &backup); err != nil {
		return fmt.Errorf("read backup: %w", err)
	}
	if backup.Wallpaper != "" {
		if _, err := os.Stat(backup.Wallpaper); err == nil {
			p := u16Ptr(backup.Wallpaper)
			if p != nil {
				_ = systemParametersInfoW(spiSetDeskWallpaper, 0, p)
			}
		}
	}
	_ = os.Remove(backupPath())
	return nil
}
