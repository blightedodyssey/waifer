// Package platform centralizes OS-specific paths and small file/process
// helpers used by the rest of the application.
package platform

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// homeDir returns the user's home directory (~).
func homeDir() string {
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return home
}

// xdgDir resolves an XDG base-directory variable, falling back to a default
// under the home directory. Only meaningful on Linux/BSD.
func xdgDir(env, fallbackRel string) string {
	if v := os.Getenv(env); v != "" {
		return filepath.Join(v, "aether")
	}
	return filepath.Join(homeDir(), fallbackRel, "aether")
}

// ConfigDir is where Aether keeps settings, blueprints and generated themes.
// Windows: %APPDATA%\aether. Linux: XDG_CONFIG_HOME or ~/.config/aether.
func ConfigDir() string {
	switch runtime.GOOS {
	case "windows":
		if v := os.Getenv("APPDATA"); v != "" {
			return filepath.Join(v, "aether")
		}
		return filepath.Join(homeDir(), "AppData", "Roaming", "aether")
	case "darwin":
		return filepath.Join(homeDir(), "Library", "Application Support", "aether")
	default:
		return xdgDir("XDG_CONFIG_HOME", ".config")
	}
}

// DataDir is where Aether keeps larger persistent files (saved themes).
// Windows: %LOCALAPPDATA%\aether. Linux: XDG_DATA_HOME or ~/.local/share/aether.
func DataDir() string {
	switch runtime.GOOS {
	case "windows":
		if v := os.Getenv("LOCALAPPDATA"); v != "" {
			return filepath.Join(v, "aether")
		}
		return filepath.Join(homeDir(), "AppData", "Local", "aether")
	case "darwin":
		return filepath.Join(homeDir(), "Library", "Application Support", "aether")
	default:
		return xdgDir("XDG_DATA_HOME", filepath.Join(".local", "share"))
	}
}

// CacheDir is for regenerable data (thumbnails, color caches, downloads).
// Windows: %LOCALAPPDATA%\aether\cache. Linux: XDG_CACHE_HOME or ~/.cache/aether.
func CacheDir() string {
	switch runtime.GOOS {
	case "windows":
		base := os.Getenv("LOCALAPPDATA")
		if base == "" {
			base = filepath.Join(homeDir(), "AppData", "Local")
		}
		return filepath.Join(base, "aether", "cache")
	case "darwin":
		return filepath.Join(homeDir(), "Library", "Caches", "aether")
	default:
		return xdgDir("XDG_CACHE_HOME", ".cache")
	}
}

// ThemeDir is the output root for the active theme (colors.toml lives here).
func ThemeDir() string {
	return filepath.Join(ConfigDir(), "theme")
}

// BlueprintDir is where saved blueprints (JSON) are stored.
func BlueprintDir() string {
	return filepath.Join(ConfigDir(), "blueprints")
}

// FavoritesFile is the favorites store (JSON).
func FavoritesFile() string {
	return filepath.Join(ConfigDir(), "favorites.json")
}

// CustomDir is where user template overrides and custom app folders live
// (~/.config/aether/custom/).
func CustomDir() string {
	return filepath.Join(ConfigDir(), "custom")
}

// SavedThemesDir is the root for standalone (non-Omarchy) saved theme folders.
func SavedThemesDir() string {
	return filepath.Join(DataDir(), "themes")
}

// OmarchyThemesDir is where Omarchy theme folders are installed. Outside a
// real Omarchy system it falls back to a data-directory equivalent so the
// export flow still works.
func OmarchyThemesDir() string {
	if runtime.GOOS == "linux" {
		return filepath.Join(homeDir(), ".local", "share", "omarchy", "themes")
	}
	return filepath.Join(DataDir(), "omarchy", "themes")
}

// OmarchyThemeDir is an alias of OmarchyThemesDir kept for compatibility with
// the original codebase.
func OmarchyThemeDir() string {
	return OmarchyThemesDir()
}

// WallpaperDir is the default local wallpaper library: ~/Wallpapers.
func WallpaperDir() string {
	return filepath.Join(homeDir(), "Wallpapers")
}

// BlurDir caches blurred wallpaper variants.
func BlurDir() string {
	return filepath.Join(CacheDir(), "blur")
}

// ThumbnailDir caches wallpaper thumbnails and previews.
func ThumbnailDir() string {
	return filepath.Join(CacheDir(), "thumbnails")
}

// ColorCacheDir caches extracted palettes keyed by image content.
func ColorCacheDir() string {
	return filepath.Join(CacheDir(), "colors")
}

// DownloadDir is the landing zone for remote wallpaper downloads.
func DownloadDir() string {
	return filepath.Join(CacheDir(), "downloads")
}

// allDirs lists every directory the application needs on startup.
func allDirs() []string {
	return []string{
		ConfigDir(),
		ThemeDir(),
		BlueprintDir(),
		CustomDir(),
		SavedThemesDir(),
		CacheDir(),
		BlurDir(),
		ThumbnailDir(),
		ColorCacheDir(),
		DownloadDir(),
	}
}

// EnsureAllDirs creates every application directory if missing.
func EnsureAllDirs() error {
	for _, dir := range allDirs() {
		if err := EnsureDir(dir); err != nil {
			return err
		}
	}
	return nil
}

// ExpandHome replaces a leading ~ with the user's home directory.
func ExpandHome(path string) string {
	if path == "~" {
		return homeDir()
	}
	if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, "~\\") {
		return filepath.Join(homeDir(), path[2:])
	}
	return path
}
