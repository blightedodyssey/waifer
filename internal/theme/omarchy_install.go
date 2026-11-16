package theme

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"

	"aether/internal/omarchy"
)

// omarchyBinary is the Omarchy control CLI looked up on PATH.
const omarchyBinary = "omarchy"

// IsOmarchyInstalled reports whether the Omarchy CLI is available. On
// Windows this is always false, which makes every Omarchy-specific flow a
// no-op and routes the app through the standalone writer.
func IsOmarchyInstalled() bool {
	if runtime.GOOS == "windows" {
		return false
	}
	_, err := exec.LookPath(omarchyBinary)
	return err == nil
}

// validThemeNameRe matches names Omarchy's theme menu can display safely.
var validThemeNameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)

// ValidOmarchyThemeName reports whether name is safe for a theme folder.
func ValidOmarchyThemeName(name string) bool {
	return validThemeNameRe.MatchString(name)
}

// SupportsOmarchyOverride reports whether Omarchy's native theme loader
// handles the application; Aether then leaves that app's template out of the
// rendered set on Omarchy systems. Standalone systems render everything.
func SupportsOmarchyOverride(app string) bool {
	switch app {
	case "", "colors", "aether":
		return false
	}
	return omarchy.SupportsThemeOverride(app)
}

// lightModeMarker is the file dropped into the theme dir while light mode is
// active so external tools can detect the mode.
const lightModeMarker = "light-mode"

// HandleLightModeMarker writes or removes the light-mode marker in the theme
// directory to match the state.
func HandleLightModeMarker(themeDir string, lightMode bool) error {
	marker := filepath.Join(themeDir, lightModeMarker)
	if lightMode {
		if err := os.WriteFile(marker, []byte("light\n"), 0o644); err != nil {
			return err
		}
		return nil
	}
	if err := os.Remove(marker); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
