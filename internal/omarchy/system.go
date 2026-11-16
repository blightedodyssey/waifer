package omarchy

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// omarchyBinary is the CLI used to drive the desktop.
const omarchyBinary = "omarchy"

// Capabilities describes what native Omarchy integration is available.
type Capabilities struct {
	Installed bool   `json:"installed"`
	StateDir  string `json:"stateDir"`
	ThemesDir string `json:"themesDir"`
}

// IsInstalled reports whether the Omarchy CLI is on PATH.
func IsInstalled() bool {
	if runtime.GOOS == "windows" {
		return false
	}
	_, err := exec.LookPath(omarchyBinary)
	return err == nil
}

// DetectCapabilities snapshots the native integration status.
func DetectCapabilities() Capabilities {
	installed := IsInstalled()
	return Capabilities{
		Installed: installed,
		StateDir:  CurrentStateDir(),
		ThemesDir: UserThemesDir(),
	}
}

// UserThemesDir is where Omarchy stores theme folders.
func UserThemesDir() string {
	return filepath.Join(xdgDataDir(), "omarchy", "themes")
}

// CurrentStateDir is where Omarchy keeps its runtime state (theme.name).
func CurrentStateDir() string {
	return filepath.Join(xdgStateDir(), "omarchy")
}

// xdgDataDir resolves XDG_DATA_HOME (or ~/.local/share).
func xdgDataDir() string {
	if v := os.Getenv("XDG_DATA_HOME"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return filepath.Join(home, ".local", "share")
}

// xdgStateDir resolves XDG_STATE_HOME (or ~/.local/state).
func xdgStateDir() string {
	if v := os.Getenv("XDG_STATE_HOME"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return filepath.Join(home, ".local", "state")
}

// SetBackground applies a wallpaper through Omarchy:
//
//	omarchy theme bg set <path>
func SetBackground(path string) error {
	if !IsInstalled() {
		return fmt.Errorf("omarchy CLI not found")
	}
	cmd := exec.Command(omarchyBinary, "theme", "bg", "set", path)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("omarchy theme bg set: %w: %s", err, outputTail(out))
	}
	return nil
}

// ActivateTheme activates a theme through Omarchy:
//
//	omarchy theme set <name>
//
// When wallpaperPath is non-empty the background is applied first via
// SetBackground and the activation runs with OMARCHY_THEME_SKIP_BACKGROUND=1
// so Omarchy does not override it.
func ActivateTheme(name, wallpaperPath string) error {
	if !IsInstalled() {
		return fmt.Errorf("omarchy CLI not found")
	}
	if wallpaperPath != "" {
		if err := SetBackground(wallpaperPath); err != nil {
			return err
		}
	}
	cmd := exec.Command(omarchyBinary, "theme set "+name)
	if wallpaperPath != "" {
		cmd.Env = append(os.Environ(), "OMARCHY_THEME_SKIP_BACKGROUND=1")
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("omarchy theme set %s: %w: %s", name, err, outputTail(out))
	}
	return nil
}

// ActivateThemeWithInstall links a generated theme folder into Omarchy's
// theme directory before activating it.
func ActivateThemeWithInstall(themeDir, name, wallpaperPath string) error {
	link := filepath.Join(UserThemesDir(), name)
	if _, err := os.Lstat(link); err == nil {
		_ = os.Remove(link)
	}
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		return fmt.Errorf("create themes dir: %w", err)
	}
	if err := os.Symlink(themeDir, link); err != nil {
		return fmt.Errorf("install theme %s: %w", name, err)
	}
	return ActivateTheme(name, wallpaperPath)
}

// outputTail trims command output for error messages.
func outputTail(out []byte) string {
	s := string(out)
	if len(s) > 200 {
		s = s[len(s)-200:]
	}
	return s
}
