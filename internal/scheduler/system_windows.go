//go:build windows

package scheduler

import "golang.org/x/sys/windows/registry"

// systemPrefersLight reads HKCU\Software\Microsoft\Windows\CurrentVersion\Themes\Personalize
// AppsUseLightTheme. Any read failure falls back to dark.
func systemPrefersLight() bool {
	key, err := registry.OpenKey(registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer key.Close()
	value, _, err := key.GetIntegerValue("AppsUseLightTheme")
	if err != nil {
		return false
	}
	return value != 0
}
