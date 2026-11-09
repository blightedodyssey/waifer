//go:build windows

package winsys

import (
	"fmt"
	"strconv"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	dwmKey               = `Software\Microsoft\Windows\DWM`
	explorerAccentKey    = `Software\Microsoft\Windows\CurrentVersion\Explorer\Accent`
	personalizeKey       = `Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`
	colorPrevalenceValue = "ColorPrevalence"
)

var (
	user32                    = windows.NewLazySystemDLL("user32.dll")
	procSendMessageTimeoutW   = user32.NewProc("SendMessageTimeoutW")
	procSystemParametersInfoW = user32.NewProc("SystemParametersInfoW")
)

// setDWord opens keyPath for writing and stores name=value, creating the key
// if it does not exist yet.
func setDWord(keyPath, name string, value uint32) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, keyPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open %s: %w", keyPath, err)
	}
	defer k.Close()
	if err := k.SetDWordValue(name, value); err != nil {
		return fmt.Errorf("set %s\\%s: %w", keyPath, name, err)
	}
	return nil
}

// parseHexColor turns "#rrggbb" into its components.
func parseHexColor(hex string) (r, g, b uint8, err error) {
	hex = strings.TrimPrefix(strings.TrimSpace(hex), "#")
	if len(hex) != 6 {
		return 0, 0, 0, fmt.Errorf("invalid color %q", hex)
	}
	v, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid color %q: %w", hex, err)
	}
	return uint8(v >> 16), uint8(v >> 8), uint8(v), nil
}

// SetAccentColor writes the palette accent to the DWM and Explorer keys.
// Windows stores DWM colors as 0xAARRGGBB and accent colors as 0xAABBGGRR,
// so the channel order differs between the two.
func SetAccentColor(hex string) error {
	r, g, b, err := parseHexColor(hex)
	if err != nil {
		return err
	}
	argb := uint32(0xC4)<<24 | uint32(r)<<16 | uint32(g)<<8 | uint32(b)
	abgr := uint32(0xFF)<<24 | uint32(b)<<16 | uint32(g)<<8 | uint32(r)

	for name, value := range map[string]uint32{
		"ColorizationColor":        argb,
		"ColorizationAfterglow":    argb,
		"AccentColor":              abgr,
		colorPrevalenceValue:       1,
		"ColorizationColorBalance": 78,
		"ColorizationBlurBalance":  49,
	} {
		if err := setDWord(dwmKey, name, value); err != nil {
			return err
		}
	}
	if err := setDWord(personalizeKey, "EnableTransparency", 1); err != nil {
		return err
	}
	if err := setDWord(explorerAccentKey, "AccentColorMenu", abgr); err != nil {
		return err
	}
	if err := setDWord(explorerAccentKey, "StartColorMenu", abgr); err != nil {
		return err
	}
	if err := setDWord(personalizeKey, colorPrevalenceValue, 1); err != nil {
		return err
	}
	return nil
}

// SetLightMode switches the Windows app and system themes between light and
// dark mode.
func SetLightMode(light bool) error {
	value := uint32(0)
	if light {
		value = 1
	}
	for _, name := range []string{"AppsUseLightTheme", "SystemUsesLightTheme"} {
		if err := setDWord(personalizeKey, name, value); err != nil {
			return err
		}
	}
	return nil
}

// notifyColorChange tells running shell components that the color scheme
// changed. Explorer usually also needs a restart before the taskbar and
// Start menu pick up a new accent color.
func notifyColorChange() {
	const (
		hwndBroadcast       = 0xFFFF
		wmSettingChange     = 0x001A
		smtoAbortIfHung     = 0x0002
		smtoSendNotify      = 0x0000
		settingChangeTimout = 2000
	)
	for _, area := range []string{"ImmersiveColorSet", "WindowMetrics"} {
		p, err := windows.UTF16PtrFromString(area)
		if err != nil {
			continue
		}
		var result uintptr
		procSendMessageTimeoutW.Call(
			hwndBroadcast,
			wmSettingChange,
			0,
			uintptr(unsafe.Pointer(p)),
			smtoAbortIfHung|smtoSendNotify,
			settingChangeTimout,
			uintptr(unsafe.Pointer(&result)),
		)
	}
}

// applySystemColors applies accent and light/dark mode in one operation.
func applySystemColors(vars map[string]string, lightMode bool) []error {
	var errs []error
	if accent := vars["accent"]; accent != "" {
		if err := SetAccentColor(accent); err != nil {
			errs = append(errs, fmt.Errorf("accent color: %w", err))
		}
	}
	if err := SetLightMode(lightMode); err != nil {
		errs = append(errs, fmt.Errorf("light/dark mode: %w", err))
	}
	notifyColorChange()
	return errs
}
