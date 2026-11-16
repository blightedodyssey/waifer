package theme

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"aether/internal/omarchy"
	"aether/internal/platform"
	"aether/internal/winsys"
)

// ClearTheme removes the Aether-generated theme and reverts the system to
// the pre-Aether personalization:
//
//   - the active theme directory (ThemeDir) is emptied of generated files
//   - editor integrations pointing at Aether are deselected
//   - on Windows the wallpaper/accent backup is restored via winsys.Restore
//   - on Omarchy the native state is reverted through the omarchy CLI
//
// Errors from individual cleanup steps are joined so one failure does not
// block the rest of the teardown.
func ClearTheme() error {
	themeDir := platform.ThemeDir()
	var errs []error

	// Deselect editor integrations first so they do not reference files we
	// are about to delete.
	if err := clearEditorIntegrations(); err != nil {
		errs = append(errs, err)
	}

	// Remove generated files from the theme directory (keep the directory
	// itself: Aether's config lives elsewhere and colors.toml may be
	// watched by external tools).
	if err := clearThemeDir(themeDir); err != nil {
		errs = append(errs, err)
	}

	// Revert system personalization.
	if runtime.GOOS == "windows" {
		if err := winsys.Restore(); err != nil {
			errs = append(errs, fmt.Errorf("restore system: %w", err))
		}
	} else if omarchy.IsInstalled() {
		if err := clearOmarchyTheme(); err != nil {
			errs = append(errs, err)
		}
	}

	switch len(errs) {
	case 0:
		return nil
	case 1:
		return errs[0]
	default:
		return fmt.Errorf("clear theme: %v", errs)
	}
}

// clearThemeDir removes regular files at the top level of dir plus the
// backgrounds folder. Nested directories from custom apps are removed too.
func clearThemeDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read theme dir: %w", err)
	}
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		if entry.IsDir() {
			if err := os.RemoveAll(path); err != nil {
				return fmt.Errorf("remove %s: %w", path, err)
			}
			continue
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove %s: %w", path, err)
		}
	}
	return nil
}

// clearEditorIntegrations deselects the Aether theme in editors that may
// reference it. Missing settings files are ignored.
func clearEditorIntegrations() error {
	if runtime.GOOS != "windows" {
		return nil // editor integration is Windows-only in this port
	}
	var errs []error
	if err := winsys.ClearZed(); err != nil {
		errs = append(errs, err)
	}
	if err := winsys.ClearNeovim(); err != nil {
		errs = append(errs, err)
	}
	switch len(errs) {
	case 0:
		return nil
	case 1:
		return errs[0]
	default:
		return fmt.Errorf("editors: %v", errs)
	}
}

// clearOmarchyTheme removes Aether's marker from Omarchy's state dir on
// non-Windows systems.
func clearOmarchyTheme() error {
	marker := filepath.Join(omarchy.CurrentStateDir(), "theme.name")
	if err := os.Remove(marker); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("clear omarchy theme marker: %w", err)
	}
	return nil
}
