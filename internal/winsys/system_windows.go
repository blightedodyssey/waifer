//go:build windows

package winsys

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32                    = windows.NewLazySystemDLL("user32.dll")
	procSendMessageTimeoutW   = user32.NewProc("SendMessageTimeoutW")
	procSystemParametersInfoW = user32.NewProc("SystemParametersInfoW")
	procSetSysColors          = user32.NewProc("SetSysColors")
)

// SetAccentColor writes the palette accent to the DWM and Explorer keys and
// refreshes the shell. It is kept as a small public entry point for callers
// that only want to change the accent.
func SetAccentColor(hex string) error {
	values, err := accentThemeValues(map[string]string{"accent": hex})
	if err != nil {
		return err
	}
	values = append(values,
		dwordValue(personalizeKey, colorPrevalenceKey, 1),
		dwordValue(personalizeKey, "EnableTransparency", 1),
	)
	if err := writeThemeRegistry(values); err != nil {
		return err
	}
	notifyColorChange()
	return nil
}

// SetLightMode switches the Windows app and system themes between light and
// dark mode.
func SetLightMode(light bool) error {
	value := uint32(0)
	if light {
		value = 1
	}
	values := []registryValue{
		dwordValue(personalizeKey, "AppsUseLightTheme", value),
		dwordValue(personalizeKey, "SystemUsesLightTheme", value),
	}
	if err := writeThemeRegistry(values); err != nil {
		return err
	}
	notifyColorChange()
	return nil
}

// applySystemColors applies accent, light/dark mode and Chrome's accent policy
// in one atomic registry transaction, then broadcasts the change immediately.
// The transaction runs off the caller's goroutine so wallpaper and editor work
// can proceed in parallel.
func applySystemColors(vars map[string]string, lightMode bool) []error {
	values, err := systemThemeValues(vars, lightMode)
	if err != nil {
		return []error{fmt.Errorf("system colors: %w", err)}
	}

	done := make(chan error, 1)
	go func() {
		done <- writeThemeRegistry(values)
	}()
	if err := <-done; err != nil {
		return []error{fmt.Errorf("system colors: %w", err)}
	}

	// Commit has succeeded; tell Explorer, Start, the notification center and
	// every other top-level window to repaint with the new colors.
	notifyColorChange()

	// Layer 3: also invoke Win32 SetSysColors so classic Win32 controls update
	// live without a reboot.
	invokeSetSysColors(vars)
	return nil
}

// invokeSetSysColors calls the user32 SetSysColors API with the active accent
// so legacy Win32 controls (common dialogs, property sheets, etc.) repaint
// immediately. This complements the registry-based Control Panel\Colors sync.
func invokeSetSysColors(vars map[string]string) {
	accent := normalizeHex(vars["accent"])
	if accent == "" {
		accent = "#cba6f7"
	}
	r, g, b, err := parseHexColor(accent)
	if err != nil {
		return
	}
	color := uint32((uint32(b) << 16) | (uint32(g) << 8) | uint32(r))

	// Constants from WinUser.h
	const (
		colorHighlight     = 13 // COLOR_HIGHLIGHT
		colorHighlightText = 14 // COLOR_HIGHLIGHTTEXT
		colorHotTracking   = 26 // COLOR_HOTLIGHT
	)

	colors := []int32{colorHighlight, colorHighlightText, colorHotTracking}
	values := []uint32{color, color, color}

	procSetSysColors.Call(
		uintptr(len(colors)),
		uintptr(unsafe.Pointer(&colors[0])),
		uintptr(unsafe.Pointer(&values[0])),
	)
}

// notifyThemeChange broadcasts WM_SETTINGCHANGE with the "Theme" area so the
// Personalization panel and shell reload the active .theme file immediately.
func notifyThemeChange() {
	const (
		hwndBroadcast   = 0xFFFF
		wmSettingChange = 0x001A
		smtoAbortIfHung = 0x0002
		timeoutMS       = 5000
	)
	area, err := windows.UTF16PtrFromString("Theme")
	if err != nil {
		return
	}
	var result uintptr
	procSendMessageTimeoutW.Call(
		hwndBroadcast,
		wmSettingChange,
		0,
		uintptr(unsafe.Pointer(area)),
		smtoAbortIfHung,
		timeoutMS,
		uintptr(unsafe.Pointer(&result)),
	)
}

// notifyColorChange broadcasts WM_SETTINGCHANGE (plus the theme/color messages
// some legacy windows listen for) without restarting Explorer. SendMessageTimeout
// keeps hung applications from blocking the apply operation.
func notifyColorChange() {
	const (
		hwndBroadcast              = 0xFFFF
		wmSettingChange            = 0x001A
		wmThemeChanged             = 0x031A
		wmSysColorChange           = 0x0015
		wmDwmColorizationColorChanged = 0x0320
		smtoAbortIfHung            = 0x0002
		timeoutMS                  = 5000
	)

	broadcast := func(message uintptr, area string) {
		var areaPtr *uint16
		if area != "" {
			var err error
			areaPtr, err = windows.UTF16PtrFromString(area)
			if err != nil {
				return
			}
		}
		var result uintptr
		procSendMessageTimeoutW.Call(
			hwndBroadcast,
			message,
			0,
			uintptr(unsafe.Pointer(areaPtr)),
			smtoAbortIfHung,
			timeoutMS,
			uintptr(unsafe.Pointer(&result)),
		)
	}

	// Standard areas that Explorer, Start Menu, and Notification Center listen to.
	for _, area := range []string{"ImmersiveColorSet", "ImmersiveColorPrevalence", "WindowMetrics", "WindowsThemeElement"} {
		broadcast(wmSettingChange, area)
	}
	broadcast(wmThemeChanged, "")
	broadcast(wmSysColorChange, "")

	// DWM colorization change — tells File Explorer titlebars and borders to repaint.
	broadcast(wmDwmColorizationColorChanged, "")
}
