package winsys

// dwm_windows.go applies DWM (Desktop Window Manager) title-bar and border
// color attributes to running top-level windows.  It is a best-effort Windows
// 11 QoL enhancement that works without Windhawk or third-party themers.

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	dwmapi                        = windows.NewLazySystemDLL("dwmapi.dll")
	procDwmSetWindowAttribute     = dwmapi.NewProc("DwmSetWindowAttribute")
	procDwmGetWindowAttribute     = dwmapi.NewProc("DwmGetWindowAttribute")
	user32IsWindowVisible         = user32.NewProc("IsWindowVisible")
	user32EnumWindows             = user32.NewProc("EnumWindows")
	user32GetClassNameW           = user32.NewProc("GetClassNameW")
)

// DWM window attribute constants (undocumented, stable since Win10 1809).
const (
	dwmwaUseImmersiveDarkMode = 20
	dwmwaCaptionColor         = 35
	dwmwaTextColor            = 36
	dwmwaBorderColor          = 34
)

// ApplyDWMTitlebarAttributes enumerates visible top-level windows and applies
// the Aether accent as caption / border colors.  hwndFilter, if non-nil, only
// touches windows whose class name contains one of the strings (e.g. "Chrome",
// "CabinetWClass").  An empty filter applies to all windows.
func ApplyDWMTitlebarAttributes(accent string, hwndFilter []string) error {
	r, g, b, err := parseHexColor(accent)
	if err != nil {
		return fmt.Errorf("invalid accent for DWM: %w", err)
	}
	// DWM expects 0x00BBGGRR
	dwmColor := uint32((uint32(b) << 16) | (uint32(g) << 8) | uint32(r))

	enumFunc := syscall.NewCallback(func(hwnd syscall.Handle, _ uintptr) uintptr {
		if !isWindowVisible(windows.HWND(hwnd)) {
			return 1
		}
		if len(hwndFilter) > 0 {
			class := windowClassName(windows.HWND(hwnd))
			match := false
			for _, f := range hwndFilter {
				if containsIgnoreCase(class, f) {
					match = true
					break
				}
			}
			if !match {
				return 1
			}
		}

		// Apply caption (titlebar) and border colors.
		setDwmUint32(windows.HWND(hwnd), dwmwaCaptionColor, dwmColor)
		setDwmUint32(windows.HWND(hwnd), dwmwaBorderColor, dwmColor)

		// Dark mode for caption text when background is dark.
		setDwmUint32(windows.HWND(hwnd), dwmwaUseImmersiveDarkMode, 1)

		// Force a repaint of the non-client area.
		pSendMessage := user32.NewProc("SendMessageW")
		if pSendMessage.Find() == nil {
			pSendMessage.Call(uintptr(hwnd), 0x0085, 0, 0) // WM_NCPAINT
		}
		return 1
	})

	user32EnumWindows.Call(enumFunc, 0)
	return nil
}

func setDwmUint32(hwnd windows.HWND, attr uint32, value uint32) {
	procDwmSetWindowAttribute.Call(
		uintptr(hwnd),
		uintptr(attr),
		uintptr(unsafe.Pointer(&value)),
		uintptr(4),
	)
}

func isWindowVisible(hwnd windows.HWND) bool {
	ret, _, _ := user32IsWindowVisible.Call(uintptr(hwnd))
	return ret != 0
}

func windowClassName(hwnd windows.HWND) string {
	var buf [256]uint16
	_, _, _ = user32GetClassNameW.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&buf[0])), 256)
	return windows.UTF16ToString(buf[:])
}

func containsIgnoreCase(s, substr string) bool {
	return containsString(toLower(s), toLower(substr))
}

func toLower(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 'a' - 'A'
		}
	}
	return string(b)
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func equalRunes(a, b []rune) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
