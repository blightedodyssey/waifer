//go:build windows

package winsys

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	spiSetDeskWallpaper = 0x0014
	spifUpdateIniFile   = 0x0001
	spifSendChange      = 0x0002
)

// SetWallpaper copies path into Aether's managed wallpaper directory and sets
// it as the desktop background, using "fill" scaling like the Windows
// personalization settings do.
func SetWallpaper(path string) error {
	src, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve wallpaper: %w", err)
	}
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("inspect wallpaper: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("wallpaper is not a regular file")
	}

	dst := managedWallpaperPath(src)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("create wallpaper directory: %w", err)
	}
	if !strings.EqualFold(src, dst) {
		if err := copyFile(src, dst); err != nil {
			return fmt.Errorf("copy wallpaper: %w", err)
		}
	}

	p, err := windows.UTF16PtrFromString(dst)
	if err != nil {
		return err
	}
	r, _, callErr := procSystemParametersInfoW.Call(
		spiSetDeskWallpaper,
		0,
		uintptr(unsafe.Pointer(p)),
		spifUpdateIniFile|spifSendChange,
	)
	if r == 0 {
		return fmt.Errorf("SystemParametersInfoW: %w", callErr)
	}

	k, _, err := registry.CreateKey(registry.CURRENT_USER, `Control Panel\Desktop`, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open desktop settings: %w", err)
	}
	defer k.Close()
	if err := k.SetStringValue("Wallpaper", dst); err != nil {
		return fmt.Errorf("store wallpaper path: %w", err)
	}
	// 10 = fill, 0 = not tiled.
	if err := k.SetStringValue("WallpaperStyle", "10"); err != nil {
		return fmt.Errorf("store wallpaper style: %w", err)
	}
	if err := k.SetStringValue("TileWallpaper", "0"); err != nil {
		return fmt.Errorf("store wallpaper tiling: %w", err)
	}
	return nil
}

// managedWallpaperPath keeps the extension so Windows picks the right decoder.
func managedWallpaperPath(src string) string {
	dir := filepath.Join(os.Getenv("LOCALAPPDATA"), "aether", "wallpapers")
	name := "current" + strings.ToLower(filepath.Ext(src))
	if name == "current" {
		name = "current.jpg"
	}
	return filepath.Join(dir, name)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	tmp := dst + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dst)
}
