package platform

import (
	"os"
	"path/filepath"
	"runtime"
)

// ConfigDir returns ~/.config/aether.
func ConfigDir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "aether")
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = filepath.Join(homeDir(), ".config")
	}
	return filepath.Join(dir, "aether")
}

// CacheDir returns ~/.cache/aether.
func CacheDir() string {
	if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
		return filepath.Join(xdg, "aether")
	}
	if runtime.GOOS == "windows" {
		if local := os.Getenv("LOCALAPPDATA"); local != "" {
			return filepath.Join(local, "aether", "cache")
		}
	}
	dir, err := os.UserCacheDir()
	if err != nil {
		dir = filepath.Join(homeDir(), ".cache")
	}
	return filepath.Join(dir, "aether")
}

// DataDir returns the platform-specific data directory for Aether.
// Linux: ~/.local/share/aether (XDG)
// macOS: ~/Library/Application Support/aether (via os.UserConfigDir)
// Windows: %LOCALAPPDATA%\aether\data
func DataDir() string {
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "aether")
	}
	if runtime.GOOS == "windows" {
		if local := os.Getenv("LOCALAPPDATA"); local != "" {
			return filepath.Join(local, "aether", "data")
		}
	}
	if runtime.GOOS == "darwin" {
		// macOS: use ~/Library/Application Support
		dir, err := os.UserConfigDir()
		if err == nil {
			return filepath.Join(dir, "aether", "data")
		}
	}
	return filepath.Join(homeDir(), ".local", "share", "aether")
}

// ThemeDir returns ~/.config/aether/theme.
func ThemeDir() string {
	return filepath.Join(ConfigDir(), "theme")
}

// BlueprintDir returns ~/.config/aether/blueprints.
func BlueprintDir() string {
	return filepath.Join(ConfigDir(), "blueprints")
}

// SavedThemesDir returns ~/.config/aether/themes.
func SavedThemesDir() string {
	return filepath.Join(ConfigDir(), "themes")
}

// FavoritesFile returns ~/.config/aether/favorites.json.
func FavoritesFile() string {
	return filepath.Join(ConfigDir(), "favorites.json")
}

// CustomDir returns ~/.config/aether/custom.
func CustomDir() string {
	return filepath.Join(ConfigDir(), "custom")
}

// OmarchyThemesDir returns Omarchy's native ~/.config/omarchy/themes path.
func OmarchyThemesDir() string {
	return filepath.Join(homeDir(), ".config", "omarchy", "themes")
}

// OmarchyThemeDir returns ~/.config/omarchy/themes/aether.
func OmarchyThemeDir() string {
	return filepath.Join(OmarchyThemesDir(), "aether")
}

// WallpaperDir returns ~/Pictures/Wallpapers on Windows (auto-creating it if
// missing) and ~/Wallpapers on other platforms.
func WallpaperDir() string {
	if runtime.GOOS == "windows" {
		dir := filepath.Join(homeDir(), "Pictures", "Wallpapers")
		_ = os.MkdirAll(dir, 0o755)
		return dir
	}
	return filepath.Join(homeDir(), "Wallpapers")
}

// DownloadDir returns ~/.local/share/aether/wallpapers.
func DownloadDir() string {
	return filepath.Join(DataDir(), "wallpapers")
}

// ThumbnailDir returns ~/.cache/aether/thumbnails.
func ThumbnailDir() string {
	return filepath.Join(CacheDir(), "thumbnails")
}

// ColorCacheDir returns ~/.cache/aether/color-cache.
func ColorCacheDir() string {
	return filepath.Join(CacheDir(), "color-cache")
}

// BlurDir returns ~/.cache/aether/blur.
func BlurDir() string {
	return filepath.Join(CacheDir(), "blur")
}

// EnsureAllDirs creates all directories required by Aether.
func EnsureAllDirs() error {
	dirs := []string{
		ConfigDir(),
		CacheDir(),
		DataDir(),
		ThemeDir(),
		BlueprintDir(),
		SavedThemesDir(),
		CustomDir(),
		WallpaperDir(),
		DownloadDir(),
		ThumbnailDir(),
		ColorCacheDir(),
		BlurDir(),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return err
		}
	}
	return nil
}

// HomeDir returns the current user's home directory, honoring $HOME in test
// environments across platforms.
func HomeDir() string {
	return homeDir()
}

// homeDir returns the current user's home directory.
func homeDir() string {
	if runtime.GOOS == "windows" {
		if home := os.Getenv("HOME"); home != "" {
			return home
		}
	}
	dir, err := os.UserHomeDir()
	if err != nil {
		if runtime.GOOS == "darwin" {
			return filepath.Join("/Users", os.Getenv("USER"))
		}
		return filepath.Join("/home", os.Getenv("USER"))
	}
	return dir
}
