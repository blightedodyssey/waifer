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
	values = append(values, chromeThemeValues(map[string]string{"accent": hex})...)
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
	return nil
}

// notifyColorChange broadcasts WM_SETTINGCHANGE (plus the theme/color messages
// some legacy windows listen for) without restarting Explorer. SendMessageTimeout
// keeps hung applications from blocking the apply operation.
func notifyColorChange() {
	const (
		hwndBroadcast    = 0xFFFF
		wmSettingChange  = 0x001A
		wmThemeChanged   = 0x031A
		wmSysColorChange = 0x0015
		smtoAbortIfHung  = 0x0002
		timeoutMS        = 5000
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

	for _, area := range []string{"ImmersiveColorSet", "ImmersiveColorPrevalence", "WindowMetrics"} {
		broadcast(wmSettingChange, area)
	}
	broadcast(wmThemeChanged, "")
	broadcast(wmSysColorChange, "")
}
