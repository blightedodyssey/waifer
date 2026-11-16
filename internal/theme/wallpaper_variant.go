package theme

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"aether/internal/platform"
	"aether/internal/wallpaper"
)

// imageExtensions lists the file extensions accepted for wallpapers and
// additional images.
var imageExtensions = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".webp": true,
	".gif": true, ".bmp": true,
}

// IsImageFile reports whether path has a supported image extension.
func IsImageFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return imageExtensions[ext]
}

// copyBackgroundDir copies srcDir's regular files into dstDir/backgrounds.
func copyBackgroundDir(srcDir, dstDir string) error {
	if srcDir == "" {
		return nil
	}
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read backgrounds dir: %w", err)
	}
	dst := filepath.Join(dstDir, "backgrounds")
	if len(entries) == 0 {
		return nil
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return fmt.Errorf("create backgrounds dir: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		src := filepath.Join(srcDir, entry.Name())
		if err := platform.CopyFile(src, filepath.Join(dst, entry.Name())); err != nil {
			return fmt.Errorf("copy background %q: %w", entry.Name(), err)
		}
	}
	return nil
}

// preserveThemeMedia re-links media from an existing theme dir into the new
// one so replacing a theme does not lose wallpapers and extra images.
func preserveThemeMedia(oldDir, newDir string) error {
	return copyBackgroundDir(oldDir, newDir)
}

// materializeWallpaper prepares the wallpaper referenced by state: the
// original is copied into the theme's backgrounds/ folder, and when blur is
// requested a blurred variant is created in the cache. Returns the effective
// wallpaper path that should be applied to the system (blurred variant when
// requested).
func materializeWallpaper(state *ThemeState, themeDir string) (string, error) {
	if state.WallpaperPath == "" {
		return "", nil
	}
	if !IsImageFile(state.WallpaperPath) {
		return "", fmt.Errorf("unsupported wallpaper file: %s", state.WallpaperPath)
	}
	if err := copyBackgroundDir(filepath.Dir(state.WallpaperPath), themeDir); err != nil {
		// A failure here is non-fatal: the wallpaper may live outside a
		// directory we can mirror wholesale.
		_ = err
	}
	if err := os.MkdirAll(filepath.Join(themeDir, "backgrounds"), 0o755); err != nil {
		return "", fmt.Errorf("create backgrounds dir: %w", err)
	}
	if err := platform.CopyFile(state.WallpaperPath,
		filepath.Join(themeDir, "backgrounds", filepath.Base(state.WallpaperPath))); err != nil {
		return "", fmt.Errorf("copy wallpaper %q: %w", state.WallpaperPath, err)
	}

	effective := state.WallpaperPath
	if state.WallpaperBlur {
		blurred, err := wallpaper.CreateBlurredVariant(state.WallpaperPath, platform.BlurDir())
		if err != nil {
			return "", err
		}
		effective = blurred
	}
	return effective, nil
}

// writeThemePreview renders a small preview image of the palette into the
// theme directory so file managers show what the theme looks like.
func writeThemePreview(themeDir string, palette [16]string) error {
	if err := wallpaper.WritePreview(filepath.Join(themeDir, "preview.png"), palette[:]); err != nil {
		return fmt.Errorf("write preview: %w", err)
	}
	return nil
}
