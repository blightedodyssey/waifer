package wallpaper

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"aether/internal/platform"
)

// WallpaperInfo describes one discovered local wallpaper.
type WallpaperInfo struct {
	Path string `json:"path"`
	Name string `json:"name"`
	Size int64  `json:"size"`
}

// ScanDefaultDirs scans the user's wallpaper library (~/Wallpapers) plus any
// folder configured as an additional library, returning image files sorted
// by path.
func ScanDefaultDirs() ([]WallpaperInfo, error) {
	return ScanDirectory(platform.WallpaperDir())
}

// ScanDirectory walks dir recursively and returns every supported image.
// Name is the path relative to dir with forward slashes.
func ScanDirectory(dir string) ([]WallpaperInfo, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("access wallpaper folder: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("wallpaper folder is not a directory: %s", dir)
	}

	var out []WallpaperInfo
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Skip Aether's own installed defaults to avoid duplicates.
			if path != dir && d.Name() == "Aether" {
				return fs.SkipDir
			}
			return nil
		}
		if !isImageFile(path) {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return nil
		}
		out = append(out, WallpaperInfo{
			Path: path,
			Name: rel,
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan wallpaper folder: %w", err)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

// IsImageFile reports whether path has a supported image extension.
func IsImageFile(path string) bool {
	return isImageFile(path)
}

// sanitizeName keeps a URL/path filename safe for the filesystem.
func sanitizeName(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	name = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		case r == '.', r == '-', r == '_':
			return r
		default:
			return '-'
		}
	}, name)
	name = strings.Trim(name, "-.")
	if name == "" {
		name = "wallpaper"
	}
	return name
}
