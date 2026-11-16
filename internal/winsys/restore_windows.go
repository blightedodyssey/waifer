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

	if err := restoreChromeTheme(); err != nil {
		errs = append(errs, fmt.Errorf("Chrome: %w", err))
	}

	if err := restoreFirefoxTheme(); err != nil {
		errs = append(errs, fmt.Errorf("Firefox: %w", err))
	}

	if err := restoreDiscordTheme(); err != nil {
		errs = append(errs, fmt.Errorf("Discord: %w", err))
	}

	if err := restoreObsidianTheme(); err != nil {
		errs = append(errs, fmt.Errorf("Obsidian: %w", err))
	}

	// Remove the generated editor integrations that Aether owns.
	if home, err := os.UserHomeDir(); err == nil {
		for _, extDir := range []string{".vscode", ".cursor"} {
			extPath := filepath.Join(home, extDir, "extensions", "local.theme-aether-1.0.0")
			if err := os.RemoveAll(extPath); err != nil {
				errs = append(errs, fmt.Errorf("%s extension: %w", extDir, err))
			}
		}
	}
	for _, appName := range []string{"Code", "Cursor"} {
		settingsPath := filepath.Join(appDataDir(), appName, "User", "settings.json")
		if err := restoreBackup(settingsPath); err != nil {
			errs = append(errs, fmt.Errorf("%s settings: %w", appName, err))
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
