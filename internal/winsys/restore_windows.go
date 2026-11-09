//go:build windows

package winsys

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Restore puts back the Windows Terminal and Zed settings that Aether edited,
// using the backups written on first apply. System colors are left as they are
// because Windows does not expose the previous values reliably.
func Restore() error {
	var errs []error

	for _, settingsPath := range terminalSettingsCandidates() {
		if err := restoreBackup(settingsPath); err != nil {
			errs = append(errs, fmt.Errorf("Windows Terminal: %w", err))
		}
	}

	zedSettings := filepath.Join(appDataDir(), "Zed", "settings.json")
	if err := restoreBackup(zedSettings); err != nil {
		errs = append(errs, fmt.Errorf("Zed: %w", err))
	}

	// Remove the generated editor integrations that Aether owns.
	if home, err := os.UserHomeDir(); err == nil {
		vscodeExt := filepath.Join(home, ".vscode", "extensions", "local.theme-aether-1.0.0")
		if err := os.RemoveAll(vscodeExt); err != nil {
			errs = append(errs, fmt.Errorf("VS Code extension: %w", err))
		}
	}
	if dir := neovimConfigDir(); dir != "" {
		for _, file := range []string{
			filepath.Join(dir, "colors", "aether.lua"),
			filepath.Join(dir, "lua", "plugins", "aether.lua"),
		} {
			if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
				errs = append(errs, fmt.Errorf("remove %s: %w", file, err))
			}
		}
	}

	return errors.Join(errs...)
}

func restoreBackup(settingsPath string) error {
	backup := settingsPath + ".aether-backup"
	if _, err := os.Stat(backup); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if err := copyFile(backup, settingsPath); err != nil {
		return err
	}
	return os.Remove(backup)
}
