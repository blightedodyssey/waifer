//go:build windows

package winsys

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// Registry locations for Windows personalization.
const (
	dwmKey         = `Software\Microsoft\Windows\DWM`
	personalizeKey = `Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`
)

// user32 syscalls.
var (
	user32                    = windows.NewLazySystemDLL("user32.dll")
	procSystemParametersInfoW = user32.NewProc("SystemParametersInfoW")
	procSendMessageTimeoutW   = user32.NewProc("SendMessageTimeoutW")
)

const (
	spiSetDeskWallpaper = 0x0014
	hwndBroadcast       = uintptr(0xFFFF)
	smtoAbortIfHung     = 0x0002
)

// parseHexColor converts #rrggbb into a COLORREF (0x00BBGGRR).
func parseHexColor(hex string) (uint32, error) {
	if len(hex) < 7 || hex[0] != '#' {
		return 0, fmt.Errorf("invalid hex color: %s", hex)
	}
	var r, g, b uint32
	if _, err := fmt.Sscanf(hex, "#%02x%02x%02x", &r, &g, &b); err != nil {
		return 0, fmt.Errorf("parse hex color %s: %w", hex, err)
	}
	return b<<16 | g<<8 | r, nil
}

// setDWord stores a DWORD value in a registry key.
func setDWord(path, name string, value uint32) error {
	key, err := registry.OpenKey(registry.CURRENT_USER, path, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer key.Close()
	return key.SetDWordValue(name, value)
}

// notifyColorChange tells the shell (and every listener) that the
// immersive color set changed, so open windows repaint without a logoff.
func notifyColorChange() {
	value, err := syscall.UTF16PtrFromString("ImmersiveColorSet")
	if err != nil {
		return
	}
	_, _, _ = procSendMessageTimeoutW.Call(
		hwndBroadcast,
		0x001A, // WM_SETTINGCHANGE
		0,
		uintptr(unsafe.Pointer(value)),
		smtoAbortIfHung,
		1000,
		0,
	)
}

// appDataDir returns %APPDATA% (Roaming).
func appDataDir() string {
	if v := os.Getenv("APPDATA"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "AppData", "Roaming")
}

// SetAccentColor sets the window accent (DWM ColorizationColor) from a hex
// color and repaints the shell.
func SetAccentColor(hex string) error {
	colorref, err := parseHexColor(hex)
	if err != nil {
		return err
	}
	// COLORREF -> RGBA colorization: keep full opacity.
	abgr := colorref | 0xFF000000
	if err := setDWord(dwmKey, "ColorizationColor", abgr); err != nil {
		return fmt.Errorf("set accent color: %w", err)
	}
	if err := setDWord(dwmKey, "ColorizationPreopaque", 1); err != nil {
		return fmt.Errorf("set accent opacity mode: %w", err)
	}
	notifyColorChange()
	return nil
}

// SetLightMode switches the system and app theme (true = light).
func SetLightMode(light bool) error {
	value := uint32(1)
	if !light {
		value = 0
	}
	if err := setDWord(personalizeKey, "AppsUseLightTheme", value); err != nil {
		return fmt.Errorf("set AppsUseLightTheme: %w", err)
	}
	if err := setDWord(personalizeKey, "SystemUsesLightTheme", value); err != nil {
		return fmt.Errorf("set SystemUsesLightTheme: %w", err)
	}
	notifyColorChange()
	return nil
}

// applySystemColors applies accent color and light/dark mode for the palette.
func applySystemColors(palette [16]string, lightMode bool) error {
	if err := SetAccentColor(palette[4]); err != nil {
		return err
	}
	return SetLightMode(lightMode)
}

// Apply applies the palette to the Windows system: accent color and mode.
func Apply(palette [16]string, lightMode bool) error {
	return applySystemColors(palette, lightMode)
}

// Restore reverts personalization to the values captured before the last
// apply.
func Restore() error {
	return restoreBackup()
}

// systemParametersInfoW wraps the wallpaper syscall.
func systemParametersInfoW(action uint32, param uintptr, data *uint16) error {
	const spiFUpdateIniFile = 0x01
	const spiFSendChange = 0x02
	ret, _, err := procSystemParametersInfoW.Call(
		uintptr(action),
		param,
		uintptr(unsafe.Pointer(data)),
		uintptr(spiFUpdateIniFile|spiFSendChange),
	)
	if ret == 0 {
		if err != nil && err.Error() != "The operation completed successfully." {
			return err
		}
		return fmt.Errorf("SystemParametersInfoW failed")
	}
	return nil
}

// u16Ptr is a convenience wrapper.
func u16Ptr(s string) *uint16 {
	p, err := windows.UTF16PtrFromString(s)
	if err != nil {
		return nil
	}
	return p
}
